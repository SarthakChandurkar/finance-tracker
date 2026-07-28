package domain

import (
	"math"
	"time"

	"financetracker/internal/models"
	"financetracker/internal/storage"
)

// ScheduleService runs the two time-based operations A2.1 describes:
// crediting each Monthly quota's allocation at the start of the month, and
// sweeping unspent Monthly balances into Global quotas at month-end. In a
// real deployment, main.go will call these once a day (checking "did we
// already do this for the current month?") — see the idempotency markers
// below.
type ScheduleService struct {
	store *storage.Store
}

func NewScheduleService(store *storage.Store) *ScheduleService {
	return &ScheduleService{store: store}
}

// Everything below tags its system-generated transactions with a marker
// string in Details, like "system:month-start:2026-07". Before doing
// anything for a given quota+month, we check whether that marker already
// exists — this makes RunMonthStart/RunMonthEndSweep safe to call more
// than once (e.g. the app restarts twice in the same day): it just skips
// quotas it's already processed for the current month instead of
// double-crediting them.
func monthKey(t time.Time) string           { return t.Format("2006-01") }
func monthStartMarker(now time.Time) string { return "system:month-start:" + monthKey(now) }
func eomSweepMarker(now time.Time) string   { return "system:eom-sweep:" + monthKey(now) }

// loanRepaymentPrefix tags a SelfTransfer as an automatic A2.6 loan
// repayment (rather than a user-initiated transfer), so
// outstandingLoanBalance can find and subtract it later. The lender's ID
// is appended so a quota that owes multiple lenders keeps each balance
// separate.
const loanRepaymentPrefix = "system:loan-repayment:"

func loanRepaymentMarker(lenderID string) string { return loanRepaymentPrefix + lenderID }

// RunMonthStart implements A2.1's "designated allocation amount is
// credited to each Monthly Only quota entity" — plus A2.6's repayment
// rule layered on top: "that allocation is first used to auto-repay the
// lending entity, before any remainder becomes available for spending."
func (s *ScheduleService) RunMonthStart(now time.Time) error {
	return s.store.Update(func(d *storage.Data) error {
		marker := monthStartMarker(now)

		for _, q := range d.Quotas {
			if !q.IsMonthly() {
				continue
			}
			if alreadyTagged(d, models.Credit, q.ID, marker) {
				continue // already ran for this quota this month
			}

			// 1. Credit the allocation. This is an audit-trail record —
			// MonthlyBreakdown (quota_service.go) still reads Allocated
			// straight from quota.MonthlyAllocation, not by summing these,
			// so this doesn't change any balance math; it just makes "this
			// quota was allocated ₹X in July" a real, queryable fact in
			// the single transaction log, per A1's Single Schema Principle.
			d.Transactions = append(d.Transactions, models.Transaction{
				ID:                 newID(),
				Type:               models.Credit,
				Amount:             q.MonthlyAllocation,
				DestinationQuotaID: q.ID,
				Date:               now,
				Details:            marker,
			})

			// 2. Auto-repay any Inter-Quota Loans this quota still owes,
			// out of the allocation that just arrived, before anything
			// else can be spent from it.
			remaining := q.MonthlyAllocation
			for _, lenderID := range lenderIDs(d, q.ID) {
				if remaining <= 0 {
					break
				}
				owed := outstandingLoanBalance(d, q.ID, lenderID)
				if owed <= 0 {
					continue
				}
				repay := math.Min(owed, remaining)

				// NOTE: this is a Monthly→Monthly SelfTransfer, which
				// validateSelfTransfer (transaction_service.go) normally
				// REJECTS for manually-entered transactions — that rule
				// exists to stop users from inventing an undocumented
				// movement. This is different: it's the system unwinding
				// an existing, already-approved loan, exactly as A2.6
				// describes, so it's written directly into the log here
				// rather than going through RecordTransaction's checks.
				d.Transactions = append(d.Transactions, models.Transaction{
					ID:                 newID(),
					Type:               models.SelfTransfer,
					Amount:             repay,
					SourceQuotaID:      q.ID,
					DestinationQuotaID: lenderID,
					Date:               now,
					Details:            loanRepaymentMarker(lenderID),
				})
				remaining -= repay
			}
		}
		return nil
	})
}

// RunMonthEndSweep implements A2.1's other half: "the remaining unspent
// balance of each Monthly Only quota entity is automatically swept into
// its linked Global Only entity (if paired), or into its explicitly set
// EOM Sweep Destination otherwise."
func (s *ScheduleService) RunMonthEndSweep(now time.Time) error {
	return s.store.Update(func(d *storage.Data) error {
		marker := eomSweepMarker(now)

		for _, q := range d.Quotas {
			if !q.IsMonthly() {
				continue
			}

			balance, err := monthlyAvailable(d, q.ID, now)
			if err != nil {
				return err
			}
			if balance <= 0 {
				// Nothing left to sweep — this is what actually makes this
				// idempotent. (An explicit "already swept this month" marker
				// check used to live here too, but it over-blocked: it
				// permanently stopped a quota from being swept again for
				// the rest of the month even after new money legitimately
				// arrived in it later. balance<=0 alone already prevents a
				// redundant no-op re-sweep, without that side effect.)
				continue
			}

			dest := q.EOMSweepDestination
			if dest == "" {
				dest = models.SavingsQuotaID // final fallback per A2.2
			}

			d.Transactions = append(d.Transactions, models.Transaction{
				ID:                 newID(),
				Type:               models.SelfTransfer,
				Amount:             balance,
				SourceQuotaID:      q.ID,
				DestinationQuotaID: dest,
				Date:               now,
				Details:            marker,
			})
		}
		return nil
	})
}

// alreadyTagged checks whether a system transaction with the given type,
// source-or-destination quota, and Details marker already exists — the
// idempotency check both scheduled operations rely on.
func alreadyTagged(d *storage.Data, txType models.TransactionType, quotaID, marker string) bool {
	for _, tx := range d.Transactions {
		if tx.Type != txType || tx.Details != marker {
			continue
		}
		if tx.DestinationQuotaID == quotaID || tx.SourceQuotaID == quotaID {
			return true
		}
	}
	return false
}

// lenderIDs returns every distinct quota that has ever lent to borrowerID
// via an Inter-Quota Loan — the set of lenders RunMonthStart needs to
// check for outstanding balances.
func lenderIDs(d *storage.Data, borrowerID string) []string {
	seen := map[string]bool{}
	var lenders []string
	for _, tx := range d.Transactions {
		if tx.Type == models.InterQuotaLoan && tx.DestinationQuotaID == borrowerID {
			if !seen[tx.SourceQuotaID] {
				seen[tx.SourceQuotaID] = true
				lenders = append(lenders, tx.SourceQuotaID)
			}
		}
	}
	return lenders
}

// outstandingLoanBalance is A2.6's per-lender running total for one
// borrower: every Inter-Quota Loan from lenderID to borrowerID, minus
// every automatic repayment (tagged via loanRepaymentMarker) already made
// back to that same lender.
func outstandingLoanBalance(d *storage.Data, borrowerID, lenderID string) float64 {
	var total float64
	repaymentMarker := loanRepaymentMarker(lenderID)
	for _, tx := range d.Transactions {
		switch {
		case tx.Type == models.InterQuotaLoan && tx.SourceQuotaID == lenderID && tx.DestinationQuotaID == borrowerID:
			total += tx.Amount
		case tx.Type == models.SelfTransfer && tx.SourceQuotaID == borrowerID && tx.DestinationQuotaID == lenderID && tx.Details == repaymentMarker:
			total -= tx.Amount
		}
	}
	if total < 0 {
		return 0
	}
	return total
}

// hasOutstandingInterQuotaLoan reports whether quotaID is currently
// involved — as either borrower or lender — in any Inter-Quota Loan that
// isn't fully repaid yet. QuotaService.DeleteQuota uses this to refuse
// deleting a quota with an unsettled loan still attached to it, since
// deleting it would otherwise silently erase that debt relationship
// (either side of it) without ever repaying it.
func hasOutstandingInterQuotaLoan(d *storage.Data, quotaID string) bool {
	// quotaID as borrower: does it still owe any lender?
	for _, lenderID := range lenderIDs(d, quotaID) {
		if outstandingLoanBalance(d, quotaID, lenderID) > 0 {
			return true
		}
	}
	// quotaID as lender: does any borrower still owe IT?
	seenBorrower := map[string]bool{}
	for _, tx := range d.Transactions {
		if tx.Type != models.InterQuotaLoan {
			continue
		}
		borrowerID := tx.DestinationQuotaID
		if seenBorrower[borrowerID] {
			continue
		}
		seenBorrower[borrowerID] = true
		if outstandingLoanBalance(d, borrowerID, quotaID) > 0 {
			return true
		}
	}
	return false
}

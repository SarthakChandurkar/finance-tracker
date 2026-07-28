package domain

import (
	"fmt"
	"time"

	"financetracker/internal/models"
	"financetracker/internal/storage"
)

// LoanLedgerEntry is a per-counterparty summary of external loan activity.
type LoanLedgerEntry struct {
	Counterparty            string  `json:"counterparty"`
	TotalDebit              float64 `json:"total_debit"`
	TotalCredit             float64 `json:"total_credit"`
	TotalLoanReceived       float64 `json:"total_loan_received"`
	OutstandingExternalLoan float64 `json:"outstanding_external_loan"`
}

// LoanService computes counterparty ledger entries from the transaction log.
type LoanService struct {
	store *storage.Store
}

func NewLoanService(store *storage.Store) *LoanService {
	return &LoanService{store: store}
}

// ledgerEntriesFromData does the actual aggregation over a *storage.Data
// snapshot. It's split out from CounterpartyLedger so other code (like
// transaction_service.go's over-settlement check) can reuse the exact same
// math from inside a store.Update closure, without calling store.View
// again (which would deadlock — see the note in balances.go).
func ledgerEntriesFromData(d *storage.Data) map[string]*LoanLedgerEntry {
	ledgerByName := map[string]*LoanLedgerEntry{}
	for _, tx := range d.Transactions {
		if tx.Counterparty == "" {
			continue
		}
		// Salary is a paycheck, never loan activity — skip it so a
		// Counterparty left on a Salary transaction doesn't create a
		// phantom entry in the loan ledger.
		if tx.Type == models.Salary {
			continue
		}
		entry, ok := ledgerByName[tx.Counterparty]
		if !ok {
			entry = &LoanLedgerEntry{Counterparty: tx.Counterparty}
			ledgerByName[tx.Counterparty] = entry
		}
		switch tx.Type {
		case models.Debit:
			entry.TotalDebit += tx.Amount
		case models.Credit, models.LoanReceived:
			if tx.Type == models.LoanReceived {
				entry.TotalLoanReceived += tx.Amount
			} else {
				entry.TotalCredit += tx.Amount
			}
		}
	}
	return ledgerByName
}

// outstandingForCounterpartyFromData is the same OutstandingExternalLoan
// math as CounterpartyLedger, for one counterparty, computed directly from
// a *storage.Data snapshot rather than via store.View.
func outstandingForCounterpartyFromData(d *storage.Data, counterparty string) float64 {
	if counterparty == "" {
		return 0
	}
	entry, ok := ledgerEntriesFromData(d)[counterparty]
	if !ok {
		return 0
	}
	outstanding := entry.TotalLoanReceived - entry.TotalDebit
	if outstanding < 0 {
		return 0
	}
	return outstanding
}

func (s *LoanService) CounterpartyLedger() ([]LoanLedgerEntry, error) {
	var ledgerByName map[string]*LoanLedgerEntry

	s.store.View(func(d storage.Data) {
		ledgerByName = ledgerEntriesFromData(&d)
	})

	entries := make([]LoanLedgerEntry, 0, len(ledgerByName))
	for _, entry := range ledgerByName {
		entry.OutstandingExternalLoan = entry.TotalLoanReceived - entry.TotalDebit
		if entry.OutstandingExternalLoan < 0 {
			entry.OutstandingExternalLoan = 0
		}
		// Once a loan is fully repaid there's nothing left to track — drop
		// it instead of leaving a permanent "outstanding 0.00" row (with a
		// still-clickable Settle button) for every counterparty who has
		// ever borrowed anything.
		if entry.OutstandingExternalLoan <= 0 {
			continue
		}
		entries = append(entries, *entry)
	}
	return entries, nil
}

func (s *LoanService) OutstandingForCounterparty(counterparty string) (float64, error) {
	if counterparty == "" {
		return 0, nil
	}
	ledger, err := s.CounterpartyLedger()
	if err != nil {
		return 0, err
	}
	for _, entry := range ledger {
		if entry.Counterparty == counterparty {
			return entry.OutstandingExternalLoan, nil
		}
	}
	return 0, nil
}

// InterQuotaLoanEntry is the A2.6 equivalent of a LoanLedgerEntry: instead
// of tracking a debt to an external counterparty, it tracks one still-owed
// Inter-Quota Loan between two quotas in the system — "borrower still owes
// lender X" — the same relationship RunMonthStart used to auto-repay
// before that automatic trigger was removed.
type InterQuotaLoanEntry struct {
	LenderQuotaID     string  `json:"lender_quota_id"`
	LenderQuotaName   string  `json:"lender_quota_name"`
	BorrowerQuotaID   string  `json:"borrower_quota_id"`
	BorrowerQuotaName string  `json:"borrower_quota_name"`
	Outstanding       float64 `json:"outstanding"`
}

// AllInterQuotaLoans lists every (lender, borrower) pair that still has an
// outstanding balance, using the exact same outstandingLoanBalance/
// lenderIDs math schedule_service.go's RunMonthStart used for automatic
// repayment.
func (s *LoanService) AllInterQuotaLoans() ([]InterQuotaLoanEntry, error) {
	var entries []InterQuotaLoanEntry
	s.store.View(func(d storage.Data) {
		seenBorrower := map[string]bool{}
		var borrowers []string
		for _, tx := range d.Transactions {
			if tx.Type != models.InterQuotaLoan {
				continue
			}
			if !seenBorrower[tx.DestinationQuotaID] {
				seenBorrower[tx.DestinationQuotaID] = true
				borrowers = append(borrowers, tx.DestinationQuotaID)
			}
		}
		for _, borrowerID := range borrowers {
			borrowerQuota, ok := findQuota(&d, borrowerID)
			if !ok {
				continue // borrower quota has since been deleted
			}
			for _, lenderID := range lenderIDs(&d, borrowerID) {
				owed := outstandingLoanBalance(&d, borrowerID, lenderID)
				if owed <= 0 {
					continue
				}
				lenderQuota, ok := findQuota(&d, lenderID)
				if !ok {
					continue // lender quota has since been deleted
				}
				entries = append(entries, InterQuotaLoanEntry{
					LenderQuotaID:     lenderID,
					LenderQuotaName:   lenderQuota.Name,
					BorrowerQuotaID:   borrowerID,
					BorrowerQuotaName: borrowerQuota.Name,
					Outstanding:       owed,
				})
			}
		}
	})
	return entries, nil
}

// SettleInterQuotaLoan lets the user manually repay part or all of an
// outstanding Inter-Quota Loan, from the borrower quota's own balance back
// to the lender. This is needed now that RunMonthStart's automatic
// repayment trigger has been removed — otherwise a loan could never be
// repaid at all.
//
// Like RunMonthStart's automatic repayment, this writes the SelfTransfer
// directly rather than going through TransactionService.RecordTransaction:
// a Monthly-borrower-to-Monthly-lender repayment is exactly the direction
// validateSelfTransfer normally rejects for manually-entered transfers
// (Self Transfer isn't a defined Monthly→Monthly pattern; Inter-Quota Loan
// is what that's for) — this is different, since it's unwinding an
// already-approved loan rather than inventing a new movement. It's tagged
// with the same loanRepaymentMarker so outstandingLoanBalance recognizes
// it exactly like an automatic repayment would.
func (s *LoanService) SettleInterQuotaLoan(borrowerID, lenderID string, amount float64, now time.Time) (models.Transaction, error) {
	if amount <= 0 {
		return models.Transaction{}, fmt.Errorf("amount must be greater than zero")
	}

	var saved models.Transaction
	err := s.store.Update(func(d *storage.Data) error {
		if _, ok := findQuota(d, borrowerID); !ok {
			return fmt.Errorf("borrower quota not found: %s", borrowerID)
		}
		if _, ok := findQuota(d, lenderID); !ok {
			return fmt.Errorf("lender quota not found: %s", lenderID)
		}

		owed := outstandingLoanBalance(d, borrowerID, lenderID)
		if amount > owed {
			return fmt.Errorf("cannot settle %.2f — only %.2f is currently outstanding", amount, owed)
		}

		bal, err := availableBalance(d, borrowerID, now)
		if err != nil {
			return err
		}
		if amount > bal {
			return fmt.Errorf("insufficient funds in quota %s: available %.2f, requested %.2f", borrowerID, bal, amount)
		}

		tx := models.Transaction{
			ID:                 newID(),
			Type:               models.SelfTransfer,
			Amount:             amount,
			SourceQuotaID:      borrowerID,
			DestinationQuotaID: lenderID,
			Date:               now,
			Details:            loanRepaymentMarker(lenderID),
		}
		d.Transactions = append(d.Transactions, tx)
		saved = tx
		return nil
	})
	if err != nil {
		return models.Transaction{}, err
	}
	return saved, nil
}

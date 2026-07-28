package domain

import (
	"fmt"
	"time"

	"financetracker/internal/models"
	"financetracker/internal/storage"
)

// findQuota looks up a quota by ID within a Data snapshot.
func findQuota(d *storage.Data, quotaID string) (models.Quota, bool) {
	for _, q := range d.Quotas {
		if q.ID == quotaID {
			return q, true
		}
	}
	return models.Quota{}, false
}

// monthlyAvailable and globalAccumulated below are DELIBERATELY NOT
// methods on QuotaService and DON'T call store.View/store.Update. Here's
// why that matters:
//
// storage.Store's mutex (sync.RWMutex) is NOT reentrant — if code holding
// a lock tries to acquire that same lock again before releasing it, the
// program deadlocks (freezes forever, waiting on itself). TransactionService
// needs to check a quota's balance WHILE it's already inside a store.Update
// (i.e. already holding the write lock) — so it cannot call
// store.View(...) again from in there. These functions take a plain
// *storage.Data instead, so they can be called either from inside a
// store.View closure OR from inside a store.Update closure safely.

func monthlyAvailable(d *storage.Data, quotaID string, now time.Time) (float64, error) {
	quota, found := findQuota(d, quotaID)
	if !found {
		return 0, fmt.Errorf("quota not found: %s", quotaID)
	}
	if !quota.IsMonthly() {
		return 0, fmt.Errorf("quota %s is not a Monthly Only quota", quotaID)
	}

	available := quota.MonthlyAllocation
	for _, tx := range d.Transactions {
		if tx.Date.Year() != now.Year() || tx.Date.Month() != now.Month() {
			continue
		}
		switch tx.Type {
		case models.Debit:
			if tx.SourceQuotaID == quotaID {
				available -= tx.Amount
			}
		case models.Credit, models.Salary, models.LoanReceived:
			if tx.DestinationQuotaID == quotaID {
				available += tx.Amount
			}
		case models.SelfTransfer:
			if tx.DestinationQuotaID == quotaID {
				available += tx.Amount
			}
			if tx.SourceQuotaID == quotaID {
				available -= tx.Amount
			}
		case models.InterQuotaLoan:
			if tx.DestinationQuotaID == quotaID {
				available += tx.Amount
			}
			if tx.SourceQuotaID == quotaID {
				available -= tx.Amount
			}
		}
	}
	return available, nil
}

func globalAccumulated(d *storage.Data, quotaID string) (float64, error) {
	quota, found := findQuota(d, quotaID)
	if !found {
		return 0, fmt.Errorf("quota not found: %s", quotaID)
	}
	if !quota.IsGlobal() {
		return 0, fmt.Errorf("quota %s is not a Global Only quota", quotaID)
	}

	var total float64
	for _, tx := range d.Transactions {
		switch tx.Type {
		case models.Debit:
			if tx.SourceQuotaID == quotaID {
				total -= tx.Amount
			}
		case models.Credit, models.Salary, models.LoanReceived:
			if tx.DestinationQuotaID == quotaID {
				total += tx.Amount
			}
		case models.SelfTransfer:
			if tx.DestinationQuotaID == quotaID {
				total += tx.Amount
			}
			if tx.SourceQuotaID == quotaID {
				total -= tx.Amount
			}
		case models.InterQuotaLoan:
			// Per A2.6, a Global Only quota can only ever be the LENDER in
			// an Inter-Quota Loan (loans only ever lend INTO a Monthly
			// quota — see validateInterQuotaLoan) — so we only need to
			// handle the source side here. Lending money out reduces the
			// lender's own accumulated balance, exactly like a Self
			// Transfer out would; it's repaid later via an automatic
			// SelfTransfer (see RunMonthStart), which the SelfTransfer
			// case above already credits back.
			if tx.SourceQuotaID == quotaID {
				total -= tx.Amount
			}
		}
	}
	return total, nil
}

// availableBalance is a small dispatcher: given ANY quota ID, return its
// current spendable balance regardless of scope.
func availableBalance(d *storage.Data, quotaID string, now time.Time) (float64, error) {
	quota, found := findQuota(d, quotaID)
	if !found {
		return 0, fmt.Errorf("quota not found: %s", quotaID)
	}
	if quota.IsMonthly() {
		return monthlyAvailable(d, quotaID, now)
	}
	return globalAccumulated(d, quotaID)
}
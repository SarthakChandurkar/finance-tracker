package domain

import (
	"math"
	"time"

	"financetracker/internal/models"
	"financetracker/internal/storage"
)

type ScheduleService struct {
	store storage.DataStore
}

func NewScheduleService(store storage.DataStore) *ScheduleService {
	return &ScheduleService{store: store}
}

func monthKey(t time.Time) string           { return t.Format("2006-01") }
func monthStartMarker(now time.Time) string { return "system:month-start:" + monthKey(now) }
func eomSweepMarker(now time.Time) string   { return "system:eom-sweep:" + monthKey(now) }

const loanRepaymentPrefix = "system:loan-repayment:"

func loanRepaymentMarker(lenderID string) string { return loanRepaymentPrefix + lenderID }

func (s *ScheduleService) RunMonthStart(now time.Time) error {
	return s.store.Update(func(d *storage.Data) error {
		marker := monthStartMarker(now)

		for _, q := range d.Wallets {
			if !q.IsMonthly() {
				continue
			}
			if alreadyTagged(d, models.Credit, q.ID, marker) {
				continue // already ran for this wallet this month
			}

			d.Transactions = append(d.Transactions, models.Transaction{
				ID:                  newID(),
				Type:                models.Credit,
				Amount:              0.00,
				DestinationWalletID: q.ID,
				Date:                now,
				Details:             marker,
			})

			remaining := 0.00
			for _, lenderID := range lenderIDs(d, q.ID) {
				if remaining <= 0 {
					break
				}
				owed := outstandingLoanBalance(d, q.ID, lenderID)
				if owed <= 0 {
					continue
				}
				repay := math.Min(owed, remaining)

				d.Transactions = append(d.Transactions, models.Transaction{
					ID:                  newID(),
					Type:                models.SelfTransfer,
					Amount:              repay,
					SourceWalletID:      q.ID,
					DestinationWalletID: lenderID,
					Date:                now,
					Details:             loanRepaymentMarker(lenderID),
				})
				remaining -= repay
			}
		}
		return nil
	})
}

func (s *ScheduleService) RunMonthEndSweep(now time.Time) error {
	return s.store.Update(func(d *storage.Data) error {
		marker := eomSweepMarker(now)

		for _, q := range d.Wallets {
			if !q.IsMonthly() {
				continue
			}

			balance, err := monthlyAvailable(d, q.ID, now)
			if err != nil {
				return err
			}
			if balance <= 0 {

				continue
			}

			dest := q.EOMSweepDestination
			if dest == "" {
				dest = models.SavingsWalletID
			}

			d.Transactions = append(d.Transactions, models.Transaction{
				ID:                  newID(),
				Type:                models.SelfTransfer,
				Amount:              balance,
				SourceWalletID:      q.ID,
				DestinationWalletID: dest,
				Date:                now,
				Details:             marker,
			})
		}
		return nil
	})
}

func alreadyTagged(d *storage.Data, txType models.TransactionType, walletID, marker string) bool {
	for _, tx := range d.Transactions {
		if tx.Type != txType || tx.Details != marker {
			continue
		}
		if tx.DestinationWalletID == walletID || tx.SourceWalletID == walletID {
			return true
		}
	}
	return false
}

func lenderIDs(d *storage.Data, borrowerID string) []string {
	seen := map[string]bool{}
	var lenders []string
	for _, tx := range d.Transactions {
		if tx.Type == models.InterWalletLoan && tx.DestinationWalletID == borrowerID {
			if !seen[tx.SourceWalletID] {
				seen[tx.SourceWalletID] = true
				lenders = append(lenders, tx.SourceWalletID)
			}
		}
	}
	return lenders
}

func outstandingLoanBalance(d *storage.Data, borrowerID, lenderID string) float64 {
	var total float64
	repaymentMarker := loanRepaymentMarker(lenderID)
	for _, tx := range d.Transactions {
		switch {
		case tx.Type == models.InterWalletLoan && tx.SourceWalletID == lenderID && tx.DestinationWalletID == borrowerID:
			total += tx.Amount
		case tx.Type == models.SelfTransfer && tx.SourceWalletID == borrowerID && tx.DestinationWalletID == lenderID && tx.Details == repaymentMarker:
			total -= tx.Amount
		}
	}
	if total < 0 {
		return 0
	}
	return total
}

func hasOutstandingInterWalletLoan(d *storage.Data, walletID string) bool {

	for _, lenderID := range lenderIDs(d, walletID) {
		if outstandingLoanBalance(d, walletID, lenderID) > 0 {
			return true
		}
	}

	seenBorrower := map[string]bool{}
	for _, tx := range d.Transactions {
		if tx.Type != models.InterWalletLoan {
			continue
		}
		borrowerID := tx.DestinationWalletID
		if seenBorrower[borrowerID] {
			continue
		}
		seenBorrower[borrowerID] = true
		if outstandingLoanBalance(d, borrowerID, walletID) > 0 {
			return true
		}
	}
	return false
}

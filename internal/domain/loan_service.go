package domain

import (
	"fmt"
	"math"
	"time"

	"financetracker/internal/models"
	"financetracker/internal/storage"
)

type LoanLedgerEntry struct {
	Counterparty            string  `json:"counterparty"`
	TotalDebit              float64 `json:"total_debit"`
	TotalCredit             float64 `json:"total_credit"`
	TotalLoanReceived       float64 `json:"total_loan_received"`
	OutstandingExternalLoan float64 `json:"outstanding_external_loan"`
}

type LoanService struct {
	store storage.DataStore
}

func NewLoanService(store storage.DataStore) *LoanService {
	return &LoanService{store: store}
}

func ledgerEntriesFromData(d *storage.Data) map[string]*LoanLedgerEntry {
	ledgerByName := map[string]*LoanLedgerEntry{}
	for _, tx := range d.Transactions {
		if tx.Counterparty == "" {
			continue
		}

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

type InterWalletLoanEntry struct {
	LenderWalletID     string  `json:"lender_wallet_id"`
	LenderWalletName   string  `json:"lender_wallet_name"`
	BorrowerWalletID   string  `json:"borrower_wallet_id"`
	BorrowerWalletName string  `json:"borrower_wallet_name"`
	Outstanding        float64 `json:"outstanding"`
}

func (s *LoanService) AllInterWalletLoans() ([]InterWalletLoanEntry, error) {
	var entries []InterWalletLoanEntry
	s.store.View(func(d storage.Data) {
		seenBorrower := map[string]bool{}
		var borrowers []string
		for _, tx := range d.Transactions {
			if tx.Type != models.InterWalletLoan {
				continue
			}
			if !seenBorrower[tx.DestinationWalletID] {
				seenBorrower[tx.DestinationWalletID] = true
				borrowers = append(borrowers, tx.DestinationWalletID)
			}
		}
		for _, borrowerID := range borrowers {
			borrowerWallet, ok := findWallet(&d, borrowerID)
			if !ok {
				continue
			}
			for _, lenderID := range lenderIDs(&d, borrowerID) {
				owed := outstandingLoanBalance(&d, borrowerID, lenderID)
				if owed <= 0 {
					continue
				}
				lenderWallet, ok := findWallet(&d, lenderID)
				if !ok {
					continue
				}
				entries = append(entries, InterWalletLoanEntry{
					LenderWalletID:     lenderID,
					LenderWalletName:   lenderWallet.Name,
					BorrowerWalletID:   borrowerID,
					BorrowerWalletName: borrowerWallet.Name,
					Outstanding:        owed,
				})
			}
		}
	})
	return entries, nil
}

func (s *LoanService) SettleInterWalletLoan(borrowerID, lenderID string, amount float64, now time.Time) (models.Transaction, error) {
	if amount <= 0 {
		return models.Transaction{}, fmt.Errorf("amount must be greater than zero")
	}

	if math.Round(amount*100)/100 != amount {
		return models.Transaction{}, fmt.Errorf("amount cannot have more than two decimal places")
	}

	var saved models.Transaction
	err := s.store.Update(func(d *storage.Data) error {
		if _, ok := findWallet(d, borrowerID); !ok {
			return fmt.Errorf("borrower wallet not found: %s", borrowerID)
		}
		if _, ok := findWallet(d, lenderID); !ok {
			return fmt.Errorf("lender wallet not found: %s", lenderID)
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
			return fmt.Errorf("insufficient funds in wallet %s: available %.2f, requested %.2f", borrowerID, bal, amount)
		}

		tx := models.Transaction{
			ID:                  newID(),
			Type:                models.SelfTransfer,
			Amount:              amount,
			SourceWalletID:      borrowerID,
			DestinationWalletID: lenderID,
			Date:                now,
			Details:             loanRepaymentMarker(lenderID),
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

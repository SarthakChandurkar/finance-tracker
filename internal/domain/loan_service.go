package domain

import (
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

func (s *LoanService) CounterpartyLedger() ([]LoanLedgerEntry, error) {
	ledgerByName := map[string]*LoanLedgerEntry{}

	s.store.View(func(d storage.Data) {
		for _, tx := range d.Transactions {
			if tx.Counterparty == "" {
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
			case models.Credit, models.Salary, models.LoanReceived:
				if tx.Type == models.LoanReceived {
					entry.TotalLoanReceived += tx.Amount
				} else {
					entry.TotalCredit += tx.Amount
				}
			}
		}
	})

	entries := make([]LoanLedgerEntry, 0, len(ledgerByName))
	for _, entry := range ledgerByName {
		entry.OutstandingExternalLoan = entry.TotalLoanReceived - entry.TotalDebit
		if entry.OutstandingExternalLoan < 0 {
			entry.OutstandingExternalLoan = 0
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

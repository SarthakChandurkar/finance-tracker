package domain

import (
	"fmt"
	"time"

	"financetracker/internal/models"
	"financetracker/internal/storage"
)

func findWallet(d *storage.Data, walletID string) (models.Wallet, bool) {
	for _, q := range d.Wallets {
		if q.ID == walletID {
			return q, true
		}
	}
	return models.Wallet{}, false
}

func monthlyAvailable(d *storage.Data, walletID string, now time.Time) (float64, error) {
	wallet, found := findWallet(d, walletID)
	if !found {
		return 0, fmt.Errorf("wallet not found: %s", walletID)
	}
	if !wallet.IsMonthly() {
		return 0, fmt.Errorf("wallet %s is not a Monthly Only wallet", walletID)
	}

	available := 0.00
	for _, tx := range d.Transactions {
		if tx.Date.Year() != now.Year() || tx.Date.Month() != now.Month() {
			continue
		}
		switch tx.Type {
		case models.Debit:
			if tx.SourceWalletID == walletID {
				available -= tx.Amount
			}
		case models.Credit, models.Salary, models.LoanReceived:
			if tx.DestinationWalletID == walletID {
				available += tx.Amount
			}
		case models.SelfTransfer:
			if tx.DestinationWalletID == walletID {
				available += tx.Amount
			}
			if tx.SourceWalletID == walletID {
				available -= tx.Amount
			}
		case models.InterWalletLoan:
			if tx.DestinationWalletID == walletID {
				available += tx.Amount
			}
			if tx.SourceWalletID == walletID {
				available -= tx.Amount
			}
		}
	}
	return available, nil
}

func globalAccumulated(d *storage.Data, walletID string) (float64, error) {
	wallet, found := findWallet(d, walletID)
	if !found {
		return 0, fmt.Errorf("wallet not found: %s", walletID)
	}
	if !wallet.IsGlobal() {
		return 0, fmt.Errorf("wallet %s is not a Global Only wallet", walletID)
	}

	var total float64
	for _, tx := range d.Transactions {
		switch tx.Type {
		case models.Debit:
			if tx.SourceWalletID == walletID {
				total -= tx.Amount
			}
		case models.Credit, models.Salary, models.LoanReceived:
			if tx.DestinationWalletID == walletID {
				total += tx.Amount
			}
		case models.SelfTransfer:
			if tx.DestinationWalletID == walletID {
				total += tx.Amount
			}
			if tx.SourceWalletID == walletID {
				total -= tx.Amount
			}
		case models.InterWalletLoan:
			if tx.SourceWalletID == walletID {
				total -= tx.Amount
			}
		}
	}
	return total, nil
}

func availableBalance(d *storage.Data, walletID string, now time.Time) (float64, error) {
	wallet, found := findWallet(d, walletID)
	if !found {
		return 0, fmt.Errorf("wallet not found: %s", walletID)
	}
	if wallet.IsMonthly() {
		return monthlyAvailable(d, walletID, now)
	}
	return globalAccumulated(d, walletID)
}

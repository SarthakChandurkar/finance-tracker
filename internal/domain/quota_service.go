package domain

import (
	"fmt"
	"time"

	"financetracker/internal/models"
	"financetracker/internal/storage"
)

type WalletService struct {
	store *storage.Store
}

func NewWalletService(store *storage.Store) *WalletService {
	return &WalletService{store: store}
}

func (s *WalletService) UpdateWallet(walletID string, name *string, targetAmount *float64, eomSweepDestination *string) (models.Wallet, error) {
	var updated models.Wallet
	err := s.store.Update(func(d *storage.Data) error {
		idx := -1
		for i, q := range d.Wallets {
			if q.ID == walletID {
				idx = i
				updated = q
				break
			}
		}
		if idx == -1 {
			return fmt.Errorf("wallet not found: %s", walletID)
		}

		if name != nil {
			updated.Name = *name
		}
		switch updated.Scope {
		case models.ScopeMonthlyOnly:
			if eomSweepDestination != nil {
				updated.EOMSweepDestination = *eomSweepDestination
			}
		case models.ScopeGlobalOnly:
			if targetAmount != nil {
				updated.TargetAmount = *targetAmount
			}
		}
		updated.ApplyDefaults(models.SavingsWalletID)
		if err := updated.Validate(); err != nil {
			return err
		}
		d.Wallets[idx] = updated
		return nil
	})
	return updated, err
}

func monthlyBreakdownFromData(d *storage.Data, walletID string, now time.Time) (MonthlyBreakdown, error) {
	result := MonthlyBreakdown{WalletID: walletID}

	var wallet models.Wallet
	var found bool
	for _, q := range d.Wallets {
		if q.ID == walletID {
			wallet = q
			found = true
			break
		}
	}
	if !found {
		return result, fmt.Errorf("wallet not found: %s", walletID)
	}
	if !wallet.IsMonthly() {
		return result, fmt.Errorf("wallet %s is not a Monthly Only wallet", walletID)
	}

	result.WalletName = wallet.Name
	for _, tx := range d.Transactions {

		if tx.Date.Year() != now.Year() || tx.Date.Month() != now.Month() {
			continue
		}
		switch tx.Type {
		case models.Debit:
			if tx.SourceWalletID == walletID {
				result.Debited += tx.Amount
			}
		case models.Credit, models.Salary, models.LoanReceived:
			if tx.DestinationWalletID == walletID {
				result.CreditsIn += tx.Amount
			}
		case models.SelfTransfer:
			if tx.DestinationWalletID == walletID {
				result.TransfersIn += tx.Amount
			}
			if tx.SourceWalletID == walletID {
				result.TransfersOut += tx.Amount
			}
		case models.InterWalletLoan:
			if tx.DestinationWalletID == walletID {
				result.LoansIn += tx.Amount
			}
			if tx.SourceWalletID == walletID {
				result.LoansOut += tx.Amount
			}
		}
	}

	result.AvailableBalance = result.Allocated + result.TransfersIn + result.LoansIn + result.CreditsIn -
		result.Debited - result.LoansOut - result.TransfersOut

	inflow := result.Allocated + result.TransfersIn + result.LoansIn + result.CreditsIn
	if inflow > 0 {
		result.DebitPercent = (result.Debited / inflow) * 100
	}
	return result, nil
}

func globalBreakdownFromData(d *storage.Data, walletID string) (GlobalBreakdown, error) {
	result := GlobalBreakdown{WalletID: walletID}

	var wallet models.Wallet
	var found bool
	for _, q := range d.Wallets {
		if q.ID == walletID {
			wallet = q
			found = true
			break
		}
	}
	if !found {
		return result, fmt.Errorf("wallet not found: %s", walletID)
	}
	if !wallet.IsGlobal() {
		return result, fmt.Errorf("wallet %s is not a Global Only wallet", walletID)
	}

	result.WalletName = wallet.Name
	for _, tx := range d.Transactions {
		switch tx.Type {
		case models.Debit:
			if tx.SourceWalletID == walletID {
				result.Accumulated -= tx.Amount
			}
		case models.Credit, models.Salary, models.LoanReceived:
			if tx.DestinationWalletID == walletID {
				result.Accumulated += tx.Amount
			}
		case models.SelfTransfer:
			if tx.DestinationWalletID == walletID {
				result.Accumulated += tx.Amount
			}
			if tx.SourceWalletID == walletID {
				result.Accumulated -= tx.Amount
			}
		case models.InterWalletLoan:

			if tx.SourceWalletID == walletID {
				result.Accumulated -= tx.Amount
			}
		}
	}

	result.HasGoal = wallet.HasGoal()
	result.TargetAmount = wallet.TargetAmount
	if result.HasGoal {
		result.PercentComplete = wallet.GoalProgress(result.Accumulated)
	}
	return result, nil
}

func (s *WalletService) AllMonthlyBreakdowns(now time.Time) ([]MonthlyBreakdown, error) {
	var results []MonthlyBreakdown
	s.store.View(func(d storage.Data) {
		for _, q := range d.Wallets {
			if !q.IsMonthly() {
				continue
			}
			breakdown, err := monthlyBreakdownFromData(&d, q.ID, now)
			if err != nil {
				continue
			}
			results = append(results, breakdown)
		}
	})
	return results, nil
}

func (s *WalletService) AllGlobalBreakdowns() ([]GlobalBreakdown, error) {
	var results []GlobalBreakdown
	s.store.View(func(d storage.Data) {
		for _, q := range d.Wallets {
			if !q.IsGlobal() {
				continue
			}
			breakdown, err := globalBreakdownFromData(&d, q.ID)
			if err != nil {
				continue
			}
			results = append(results, breakdown)
		}
	})
	return results, nil
}

func (s *WalletService) DeleteWallet(walletID string) (models.Wallet, error) {
	if walletID == models.SavingsWalletID {
		return models.Wallet{}, fmt.Errorf("the mandatory Savings wallet cannot be deleted")
	}

	var deleted models.Wallet
	err := s.store.Update(func(d *storage.Data) error {
		idx := -1
		for i, q := range d.Wallets {
			if q.ID == walletID {
				idx = i
				deleted = q
				break
			}
		}
		if idx == -1 {
			return fmt.Errorf("wallet not found: %s", walletID)
		}

		if hasOutstandingInterWalletLoan(d, walletID) {
			return fmt.Errorf("cannot delete this wallet: it has an outstanding Inter-Wallet Loan attached to it — settle it first")
		}

		balance, err := availableBalance(d, walletID, time.Now())
		if err != nil {
			return err
		}
		if balance > 0 {
			d.Transactions = append(d.Transactions, models.Transaction{
				ID:                  newID(),
				Type:                models.SelfTransfer,
				Amount:              balance,
				SourceWalletID:      walletID,
				DestinationWalletID: models.SavingsWalletID,
				Date:                time.Now(),
				Details:             "system:wallet-deletion-transfer",
			})
		}

		d.Wallets = append(d.Wallets[:idx], d.Wallets[idx+1:]...)

		if deleted.LinkedWalletID != "" {
			for j, other := range d.Wallets {
				if other.ID == deleted.LinkedWalletID {
					other.LinkedWalletID = ""
					if other.EOMSweepDestination == walletID {
						other.EOMSweepDestination = models.SavingsWalletID
					}
					d.Wallets[j] = other
					break
				}
			}
		}
		return nil
	})
	return deleted, err
}

func (s *WalletService) CreateWallet(mode models.CreationMode, name string, targetAmount float64, eomSweepDestination string) ([]models.Wallet, error) {
	switch mode {

	case models.CreateBoth:
		monthlyID := newID()
		globalID := newID()

		monthly := models.Wallet{
			ID:             monthlyID,
			Name:           name,
			Scope:          models.ScopeMonthlyOnly,
			LinkedWalletID: globalID,
		}
		global := models.Wallet{
			ID:             globalID,
			Name:           name,
			Scope:          models.ScopeGlobalOnly,
			LinkedWalletID: monthlyID,
			TargetAmount:   targetAmount,
		}

		monthly.ApplyDefaults(models.SavingsWalletID)

		if err := monthly.Validate(); err != nil {
			return nil, fmt.Errorf("monthly half: %w", err)
		}
		if err := global.Validate(); err != nil {
			return nil, fmt.Errorf("global half: %w", err)
		}

		err := s.store.Update(func(d *storage.Data) error {
			d.Wallets = append(d.Wallets, monthly, global)
			return nil
		})
		if err != nil {
			return nil, err
		}
		return []models.Wallet{monthly, global}, nil

	case models.CreateMonthlyOnly:

		q := models.Wallet{
			ID:                  newID(),
			Name:                name,
			Scope:               models.ScopeMonthlyOnly,
			EOMSweepDestination: eomSweepDestination,
		}

		q.ApplyDefaults(models.SavingsWalletID)

		if err := q.Validate(); err != nil {
			return nil, err
		}
		err := s.store.Update(func(d *storage.Data) error {
			d.Wallets = append(d.Wallets, q)
			return nil
		})
		if err != nil {
			return nil, err
		}
		return []models.Wallet{q}, nil

	case models.CreateGlobalOnly:
		q := models.Wallet{
			ID:           newID(),
			Name:         name,
			Scope:        models.ScopeGlobalOnly,
			TargetAmount: targetAmount,
		}
		if err := q.Validate(); err != nil {
			return nil, err
		}
		err := s.store.Update(func(d *storage.Data) error {
			d.Wallets = append(d.Wallets, q)
			return nil
		})
		if err != nil {
			return nil, err
		}
		return []models.Wallet{q}, nil

	default:
		return nil, fmt.Errorf("unknown creation mode: %q", mode)
	}
}

type MonthlyBreakdown struct {
	WalletID         string  `json:"wallet_id"`
	WalletName       string  `json:"wallet_name,omitempty"`
	Allocated        float64 `json:"allocated"`
	Debited          float64 `json:"debited"`
	TransfersIn      float64 `json:"transfers_in"`
	TransfersOut     float64 `json:"transfers_out"`
	LoansIn          float64 `json:"loans_in"`
	LoansOut         float64 `json:"loans_out"`
	CreditsIn        float64 `json:"credits_in"`
	AvailableBalance float64 `json:"available_balance"`
	DebitPercent     float64 `json:"debit_percent"`
}

func (s *WalletService) MonthlyBreakdown(walletID string, now time.Time) (MonthlyBreakdown, error) {
	result := MonthlyBreakdown{WalletID: walletID}

	var wallet models.Wallet
	var found bool

	s.store.View(func(d storage.Data) {
		for _, q := range d.Wallets {
			if q.ID == walletID {
				wallet = q
				found = true
				break
			}
		}
		if !found {
			return
		}
		result.WalletName = wallet.Name

		for _, tx := range d.Transactions {
			if tx.Date.Year() != now.Year() || tx.Date.Month() != now.Month() {
				continue // outside the current calendar month
			}
			switch tx.Type {
			case models.Debit:
				if tx.SourceWalletID == walletID {
					result.Debited += tx.Amount
				}
			case models.Credit, models.Salary, models.LoanReceived:
				if tx.DestinationWalletID == walletID {
					result.CreditsIn += tx.Amount
				}
			case models.SelfTransfer:
				if tx.DestinationWalletID == walletID {
					result.TransfersIn += tx.Amount
				}
				if tx.SourceWalletID == walletID {
					result.TransfersOut += tx.Amount
				}
			case models.InterWalletLoan:
				if tx.DestinationWalletID == walletID {
					result.LoansIn += tx.Amount
				}
				if tx.SourceWalletID == walletID {
					result.LoansOut += tx.Amount
				}
			}
		}
	})

	if !found {
		return result, fmt.Errorf("wallet not found: %s", walletID)
	}
	if !wallet.IsMonthly() {
		return result, fmt.Errorf("wallet %s is not a Monthly Only wallet", walletID)
	}

	result.AvailableBalance = result.Allocated + result.TransfersIn + result.LoansIn + result.CreditsIn -
		result.Debited - result.LoansOut - result.TransfersOut

	inflow := result.Allocated + result.TransfersIn + result.LoansIn + result.CreditsIn
	if inflow > 0 {
		result.DebitPercent = (result.Debited / inflow) * 100
	}

	return result, nil
}

type GlobalBreakdown struct {
	WalletID        string  `json:"wallet_id"`
	WalletName      string  `json:"wallet_name,omitempty"`
	Accumulated     float64 `json:"accumulated"`
	HasGoal         bool    `json:"has_goal"`
	TargetAmount    float64 `json:"target_amount"`
	PercentComplete float64 `json:"percent_complete"`
}

func (s *WalletService) GlobalBreakdown(walletID string) (GlobalBreakdown, error) {
	result := GlobalBreakdown{WalletID: walletID}

	var wallet models.Wallet
	var found bool

	s.store.View(func(d storage.Data) {
		for _, q := range d.Wallets {
			if q.ID == walletID {
				wallet = q
				found = true
				break
			}
		}
		if !found {
			return
		}

		for _, tx := range d.Transactions {
			switch tx.Type {
			case models.Debit:
				if tx.SourceWalletID == walletID {
					result.Accumulated -= tx.Amount
				}
			case models.Credit, models.Salary, models.LoanReceived:
				if tx.DestinationWalletID == walletID {
					result.Accumulated += tx.Amount
				}
			case models.SelfTransfer:
				if tx.DestinationWalletID == walletID {
					result.Accumulated += tx.Amount
				}
				if tx.SourceWalletID == walletID {
					result.Accumulated -= tx.Amount
				}
			case models.InterWalletLoan:

				if tx.SourceWalletID == walletID {
					result.Accumulated -= tx.Amount
				}
			}
		}
	})

	if !found {
		return result, fmt.Errorf("wallet not found: %s", walletID)
	}
	result.WalletName = wallet.Name
	if !wallet.IsGlobal() {
		return result, fmt.Errorf("wallet %s is not a Global Only wallet", walletID)
	}

	result.HasGoal = wallet.HasGoal()
	result.TargetAmount = wallet.TargetAmount
	if result.HasGoal {
		result.PercentComplete = wallet.GoalProgress(result.Accumulated)
	}

	return result, nil
}

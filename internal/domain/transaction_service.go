package domain

import (
	"fmt"
	"time"

	"financetracker/internal/models"
	"financetracker/internal/storage"
)

type TransactionService struct {
	store *storage.Store
}

func NewTransactionService(store *storage.Store) *TransactionService {
	return &TransactionService{store: store}
}

type InsufficientFundsError struct {
	WalletID  string
	Available float64
	Requested float64

	Options []FundingOption
}

func (e *InsufficientFundsError) Error() string {
	return fmt.Sprintf("insufficient funds in wallet %s: available %.2f, requested %.2f",
		e.WalletID, e.Available, e.Requested)
}

func validateSelfTransfer(source, dest models.Wallet) error {
	switch {
	case source.IsGlobal() && dest.IsMonthly():

		return nil

	case source.IsMonthly() && dest.IsGlobal():

		if dest.LinkedWalletID == source.ID {
			return fmt.Errorf("cannot manually self-transfer from a Monthly wallet into its own linked Global wallet — this happens automatically at month-end")
		}
		return nil

	case source.IsGlobal() && dest.IsGlobal():
		return nil

	case source.IsMonthly() && dest.IsMonthly():

		if source.ID == dest.ID {
			return fmt.Errorf("a wallet cannot self-transfer to itself")
		}
		if (source.IsMonthly() && source.IsGlobal()) && !(dest.IsMonthly() && dest.IsGlobal()) {
			return nil
		}

		return fmt.Errorf("a monthly wallet cannot self-transfer to another double scoped wallet (Global + Monthly)")

	default:
		return fmt.Errorf("self transfer not permitted from a %s wallet to a %s wallet", source.Scope, dest.Scope)
	}
}

func validateInterWalletLoan(source, dest models.Wallet) error {
	if !dest.IsMonthly() {

		return fmt.Errorf("inter-wallet loans can only lend into a Monthly Only wallet, not a %s wallet", dest.Scope)
	}
	if source.IsGlobal() {
		return nil
	}
	if source.IsMonthly() {
		if source.ID == dest.ID {
			return fmt.Errorf("a wallet cannot lend to itself")
		}
		if (source.IsMonthly() && source.IsGlobal()) && !(dest.IsMonthly() && dest.IsGlobal()) {
			return nil
		}
		return fmt.Errorf("a monthly wallet cannot self-transfer to another double scoped wallet (Global + Monthly)")
	}
	return fmt.Errorf("invalid loan source scope: %s", source.Scope)
}

func validateDirectionalRules(d *storage.Data, tx models.Transaction) error {
	switch tx.Type {
	case models.SelfTransfer:
		source, ok := findWallet(d, tx.SourceWalletID)
		if !ok {
			return fmt.Errorf("source wallet not found: %s", tx.SourceWalletID)
		}
		dest, ok := findWallet(d, tx.DestinationWalletID)
		if !ok {
			return fmt.Errorf("destination wallet not found: %s", tx.DestinationWalletID)
		}
		return validateSelfTransfer(source, dest)

	case models.InterWalletLoan:
		source, ok := findWallet(d, tx.SourceWalletID)
		if !ok {
			return fmt.Errorf("source wallet not found: %s", tx.SourceWalletID)
		}
		dest, ok := findWallet(d, tx.DestinationWalletID)
		if !ok {
			return fmt.Errorf("destination wallet not found: %s", tx.DestinationWalletID)
		}
		return validateInterWalletLoan(source, dest)

	default: // Debit, Credit, Salary, Loan Received
		if tx.SourceWalletID != "" {
			if _, ok := findWallet(d, tx.SourceWalletID); !ok {
				return fmt.Errorf("source wallet not found: %s", tx.SourceWalletID)
			}
		}
		if tx.DestinationWalletID != "" {
			if _, ok := findWallet(d, tx.DestinationWalletID); !ok {
				return fmt.Errorf("destination wallet not found: %s", tx.DestinationWalletID)
			}

		}
		return nil
	}
}

func checkLoanSettlementAmount(d *storage.Data, tx models.Transaction) error {
	if tx.Type != models.Debit || tx.Category != "Settle" || tx.Counterparty == "" {
		return nil
	}
	outstanding := outstandingForCounterpartyFromData(d, tx.Counterparty)
	if tx.Amount > outstanding {
		return fmt.Errorf("cannot settle %.2f for %s — only %.2f is currently outstanding", tx.Amount, tx.Counterparty, outstanding)
	}
	return nil
}

func checkNonNegativeBalance(d *storage.Data, tx models.Transaction) error {
	if tx.SourceWalletID == "" {
		return nil
	}
	bal, err := availableBalance(d, tx.SourceWalletID, tx.Date)
	if err != nil {
		return err
	}
	if bal-tx.Amount < 0 {
		options, _ := eligibleFundingOptions(d, tx.SourceWalletID, tx.Amount, tx.Date)
		return &InsufficientFundsError{
			WalletID:  tx.SourceWalletID,
			Available: bal,
			Requested: tx.Amount,
			Options:   options,
		}
	}
	return nil
}

type FundingOption struct {
	Mechanism        models.TransactionType `json:"mechanism"`
	SourceWalletID   string                 `json:"source_wallet_id"`
	SourceWalletName string                 `json:"source_wallet_name"`
	AvailableBalance float64                `json:"available_balance"`
}

func eligibleFundingOptions(d *storage.Data, targetWalletID string, amountNeeded float64, now time.Time) ([]FundingOption, error) {
	target, ok := findWallet(d, targetWalletID)
	if !ok {
		return nil, fmt.Errorf("wallet not found: %s", targetWalletID)
	}

	var options []FundingOption
	for _, candidate := range d.Wallets {
		if candidate.ID == targetWalletID {
			continue
		}

		var mechanisms []models.TransactionType
		if validateSelfTransfer(candidate, target) == nil {
			mechanisms = append(mechanisms, models.SelfTransfer)
		}
		if validateInterWalletLoan(candidate, target) == nil {
			mechanisms = append(mechanisms, models.InterWalletLoan)
		}
		if len(mechanisms) == 0 {
			continue
		}

		balance, err := availableBalance(d, candidate.ID, now)
		if err != nil {
			return nil, err
		}
		if balance < amountNeeded {
			continue
		}

		for _, mechanism := range mechanisms {
			options = append(options, FundingOption{
				Mechanism:        mechanism,
				SourceWalletID:   candidate.ID,
				SourceWalletName: candidate.Name,
				AvailableBalance: balance,
			})
		}
	}

	return options, nil
}

func (s *TransactionService) RecordTransaction(tx models.Transaction) (models.Transaction, error) {
	tx.ApplyDefaults()
	if err := tx.Validate(); err != nil {
		return models.Transaction{}, err
	}

	var saved models.Transaction
	err := s.store.Update(func(d *storage.Data) error {
		if err := validateDirectionalRules(d, tx); err != nil {
			return err
		}
		if err := checkNonNegativeBalance(d, tx); err != nil {
			return err
		}
		if err := checkLoanSettlementAmount(d, tx); err != nil {
			return err
		}
		tx.ID = newID()
		d.Transactions = append(d.Transactions, tx)
		saved = tx
		return nil
	})
	if err != nil {
		return models.Transaction{}, err
	}
	return saved, nil
}

func (s *TransactionService) EditTransaction(id string, updated models.Transaction) (models.Transaction, error) {
	updated.ID = id
	updated.ApplyDefaults()
	if err := updated.Validate(); err != nil {
		return models.Transaction{}, err
	}

	var saved models.Transaction
	err := s.store.Update(func(d *storage.Data) error {
		idx := -1
		for i, tx := range d.Transactions {
			if tx.ID == id {
				idx = i
				break
			}
		}
		if idx == -1 {
			return fmt.Errorf("transaction not found: %s", id)
		}

		without := *d
		without.Transactions = append(
			append([]models.Transaction{}, d.Transactions[:idx]...),
			d.Transactions[idx+1:]...,
		)

		if err := validateDirectionalRules(&without, updated); err != nil {
			return err
		}
		if err := checkNonNegativeBalance(&without, updated); err != nil {
			return err
		}
		if err := checkLoanSettlementAmount(&without, updated); err != nil {
			return err
		}

		d.Transactions[idx] = updated
		saved = updated
		return nil
	})
	if err != nil {
		return models.Transaction{}, err
	}
	return saved, nil
}

func (s *TransactionService) DeleteTransaction(id string) error {
	return s.store.Update(func(d *storage.Data) error {
		idx := -1
		for i, tx := range d.Transactions {
			if tx.ID == id {
				idx = i
				break
			}
		}
		if idx == -1 {
			return fmt.Errorf("transaction not found: %s", id)
		}
		d.Transactions = append(d.Transactions[:idx], d.Transactions[idx+1:]...)
		return nil
	})
}

func (s *TransactionService) EligibleFundingOptions(walletID string, amount float64, now time.Time) ([]FundingOption, error) {
	var options []FundingOption
	var err error
	s.store.View(func(d storage.Data) {
		options, err = eligibleFundingOptions(&d, walletID, amount, now)
	})
	return options, err
}

func (s *TransactionService) FundAndCreateDebit(funding FundingOption, debit models.Transaction) (models.Transaction, models.Transaction, error) {
	if debit.Type != models.Debit {
		return models.Transaction{}, models.Transaction{}, fmt.Errorf("debit transaction must be type Debit")
	}
	fundingTx := models.Transaction{
		Type:                funding.Mechanism,
		Amount:              debit.Amount,
		SourceWalletID:      funding.SourceWalletID,
		DestinationWalletID: debit.SourceWalletID,
		Date:                debit.Date,
		Details:             "system:funding-remediation",
	}
	fundingTx.ApplyDefaults()
	fundingTx.ID = newID()
	debit.ApplyDefaults()
	if err := debit.Validate(); err != nil {
		return models.Transaction{}, models.Transaction{}, err
	}
	debit.ID = newID()

	var savedFunding models.Transaction
	var savedDebit models.Transaction
	err := s.store.Update(func(d *storage.Data) error {
		if err := validateDirectionalRules(d, fundingTx); err != nil {
			return err
		}
		if err := checkNonNegativeBalance(d, fundingTx); err != nil {
			return err
		}
		d.Transactions = append(d.Transactions, fundingTx)
		if err := validateDirectionalRules(d, debit); err != nil {
			return err
		}
		if err := checkNonNegativeBalance(d, debit); err != nil {
			return err
		}
		d.Transactions = append(d.Transactions, debit)
		savedFunding = fundingTx
		savedDebit = debit
		return nil
	})
	if err != nil {
		return models.Transaction{}, models.Transaction{}, err
	}
	return savedFunding, savedDebit, nil
}

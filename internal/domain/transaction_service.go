package domain

import (
	"fmt"
	"time"

	"financetracker/internal/models"
	"financetracker/internal/storage"
)

// TransactionService is the single entry point for creating, editing, and
// deleting transactions. Every rule in A1.4, A2.5, and A2.6 is enforced
// here, in one place — this is what "Validated at transaction save time"
// in A1.4 means in code.
type TransactionService struct {
	store *storage.Store
}

func NewTransactionService(store *storage.Store) *TransactionService {
	return &TransactionService{store: store}
}

// InsufficientFundsError is a distinct, named error type (rather than just
// errors.New("insufficient funds")) so calling code can detect this
// SPECIFIC failure and react to it differently from, say, "quota not
// found." Go code checks for this with `errors.As(err, &insufficientFundsErr)`.
type InsufficientFundsError struct {
	QuotaID   string
	Available float64
	Requested float64
	// Options is A1.4's "set of eligible funding options (mechanism +
	// source quotas)" — computed right here, at rejection time, so a
	// caller (the future HTTP handler behind B2.3's prompt UI) gets
	// everything it needs to show the user in one round trip.
	Options []FundingOption
}

// Error() is the one method a type needs to satisfy Go's built-in `error`
// interface. Any type with an `Error() string` method IS an error, as far
// as Go is concerned — there's no explicit "implements" keyword needed.
func (e *InsufficientFundsError) Error() string {
	return fmt.Sprintf("insufficient funds in quota %s: available %.2f, requested %.2f",
		e.QuotaID, e.Available, e.Requested)
}

// validateSelfTransfer enforces A2.5's three allowed movement patterns.
func validateSelfTransfer(source, dest models.Quota) error {
	switch {
	case source.IsGlobal() && dest.IsMonthly():
		// Covers BOTH "Paired-Quota Replenishment" (linked pair) and the
		// unlinked "Cross-Quota Transfer" forward direction.
		return nil

	case source.IsMonthly() && dest.IsGlobal():
		// A2.5's "Cross-Quota Transfer" reverse direction is only defined
		// for a Global entity NOT linked to this Monthly one. A linked
		// pair's Monthly→Global movement happens automatically at month-end
		// (A2.1 EOM sweep) — the doc doesn't define a manual version of it,
		// so we block it here rather than silently allowing an undocumented
		// behavior.
		if dest.LinkedQuotaID == source.ID {
			return fmt.Errorf("cannot manually self-transfer from a Monthly quota into its own linked Global quota — this happens automatically at month-end")
		}
		return nil

	case source.IsGlobal() && dest.IsGlobal():
		return nil // "Global-to-Global Transfer" — always bidirectional

	default:
		// Monthly → Monthly self transfers aren't a defined pattern in
		// A2.5 at all — that's what Inter-Quota Loans are for.
		return fmt.Errorf("self transfer not permitted from a %s quota to a %s quota", source.Scope, dest.Scope)
	}
}

// validateInterQuotaLoan enforces A2.6's two allowed lending patterns.
func validateInterQuotaLoan(source, dest models.Quota) error {
	if !dest.IsMonthly() {
		// "Inter-Quota Loan does not apply here" for Global targets (A1.4,
		// A2.6) — loans only ever lend INTO a Monthly entity.
		return fmt.Errorf("inter-quota loans can only lend into a Monthly Only quota, not a %s quota", dest.Scope)
	}
	if source.IsGlobal() {
		return nil // Global-to-Monthly (Y → X)
	}
	if source.IsMonthly() {
		if source.ID == dest.ID {
			return fmt.Errorf("a quota cannot lend to itself")
		}
		return nil // Monthly-to-Monthly (Z → X)
	}
	return fmt.Errorf("invalid loan source scope: %s", source.Scope)
}

// validateDirectionalRules is the dispatcher: for Self Transfer / Inter-
// Quota Loan it checks A2.5/A2.6's movement rules; for every other type it
// just makes sure any quota IDs given actually point at real quotas.
func validateDirectionalRules(d *storage.Data, tx models.Transaction) error {
	switch tx.Type {
	case models.SelfTransfer:
		source, ok := findQuota(d, tx.SourceQuotaID)
		if !ok {
			return fmt.Errorf("source quota not found: %s", tx.SourceQuotaID)
		}
		dest, ok := findQuota(d, tx.DestinationQuotaID)
		if !ok {
			return fmt.Errorf("destination quota not found: %s", tx.DestinationQuotaID)
		}
		return validateSelfTransfer(source, dest)

	case models.InterQuotaLoan:
		source, ok := findQuota(d, tx.SourceQuotaID)
		if !ok {
			return fmt.Errorf("source quota not found: %s", tx.SourceQuotaID)
		}
		dest, ok := findQuota(d, tx.DestinationQuotaID)
		if !ok {
			return fmt.Errorf("destination quota not found: %s", tx.DestinationQuotaID)
		}
		return validateInterQuotaLoan(source, dest)

	default: // Debit, Credit, Salary, Loan Received
		if tx.SourceQuotaID != "" {
			if _, ok := findQuota(d, tx.SourceQuotaID); !ok {
				return fmt.Errorf("source quota not found: %s", tx.SourceQuotaID)
			}
		}
		if tx.DestinationQuotaID != "" {
			if _, ok := findQuota(d, tx.DestinationQuotaID); !ok {
				return fmt.Errorf("destination quota not found: %s", tx.DestinationQuotaID)
			}
		}
		return nil
	}
}

// checkNonNegativeBalance is A1.4's "no balance may go below zero" rule.
func checkNonNegativeBalance(d *storage.Data, tx models.Transaction) error {
	if tx.SourceQuotaID == "" {
		return nil // inflows don't draw from anything
	}
	bal, err := availableBalance(d, tx.SourceQuotaID, tx.Date)
	if err != nil {
		return err
	}
	if bal-tx.Amount < 0 {
		options, _ := eligibleFundingOptions(d, tx.SourceQuotaID, tx.Amount, tx.Date)
		return &InsufficientFundsError{
			QuotaID:   tx.SourceQuotaID,
			Available: bal,
			Requested: tx.Amount,
			Options:   options,
		}
	}
	return nil
}

// FundingOption is one entry in A1.4 / A2.6's "set of eligible funding
// options" — what B2.3's insufficient-funds prompt presents to the user.
type FundingOption struct {
	Mechanism        models.TransactionType `json:"mechanism"`
	SourceQuotaID    string                 `json:"source_quota_id"`
	SourceQuotaName  string                 `json:"source_quota_name"`
	AvailableBalance float64                `json:"available_balance"`
}

// eligibleFundingOptions implements A2.6's "Insufficient Funds —
// Eligibility Computation": every OTHER active quota that could legally
// fund the shortfall (per the same directional rules above) AND currently
// holds enough to cover it.
func eligibleFundingOptions(d *storage.Data, targetQuotaID string, amountNeeded float64, now time.Time) ([]FundingOption, error) {
	target, ok := findQuota(d, targetQuotaID)
	if !ok {
		return nil, fmt.Errorf("quota not found: %s", targetQuotaID)
	}

	var options []FundingOption
	for _, candidate := range d.Quotas {
		if candidate.Archived || candidate.ID == targetQuotaID {
			continue
		}

		var mechanisms []models.TransactionType
		if validateSelfTransfer(candidate, target) == nil {
			mechanisms = append(mechanisms, models.SelfTransfer)
		}
		if validateInterQuotaLoan(candidate, target) == nil {
			mechanisms = append(mechanisms, models.InterQuotaLoan)
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
				SourceQuotaID:    candidate.ID,
				SourceQuotaName:  candidate.Name,
				AvailableBalance: balance,
			})
		}
	}

	return options, nil
}

// RecordTransaction is the main "create" entry point.
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

// EditTransaction implements A1.3: "All transactions are editable after
// creation," re-validating as if the edited version were fresh.
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

		// Build a "what-if" copy of Data with the OLD version of this
		// transaction removed, so the balance check reflects things as if
		// we're inserting the edited version fresh — not double-counting
		// the original transaction we're about to replace.
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

		d.Transactions[idx] = updated
		saved = updated
		return nil
	})
	if err != nil {
		return models.Transaction{}, err
	}
	return saved, nil
}

// DeleteTransaction implements the other half of A1.3.
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

// EligibleFundingOptions is the public, read-only entry point for B2.3's
// prompt UI to ask "what could fund this shortfall?" directly.
func (s *TransactionService) EligibleFundingOptions(quotaID string, amount float64, now time.Time) ([]FundingOption, error) {
	var options []FundingOption
	var err error
	s.store.View(func(d storage.Data) {
		options, err = eligibleFundingOptions(&d, quotaID, amount, now)
	})
	return options, err
}

// FundAndCreateDebit uses a funding option to cover a shortfall and then records the target Debit.
func (s *TransactionService) FundAndCreateDebit(funding FundingOption, debit models.Transaction) (models.Transaction, models.Transaction, error) {
	if debit.Type != models.Debit {
		return models.Transaction{}, models.Transaction{}, fmt.Errorf("debit transaction must be type Debit")
	}
	fundingTx := models.Transaction{
		Type:               funding.Mechanism,
		Amount:             debit.Amount,
		SourceQuotaID:      funding.SourceQuotaID,
		DestinationQuotaID: debit.SourceQuotaID,
		Date:               debit.Date,
		Details:            "system:funding-remediation",
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

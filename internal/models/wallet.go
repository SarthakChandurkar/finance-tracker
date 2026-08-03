package models

import "errors"

type WalletScope string

const (
	ScopeMonthlyOnly WalletScope = "Monthly"
	ScopeGlobalOnly  WalletScope = "Global"
)

type CreationMode string

const (
	CreateBoth        CreationMode = "Both"
	CreateMonthlyOnly CreationMode = "Monthly"
	CreateGlobalOnly  CreationMode = "Global"
)

const SavingsWalletID = "savings"

type Wallet struct {
	ID                  string      `json:"id"`
	Name                string      `json:"name"`
	Scope               WalletScope `json:"scope"`
	LinkedWalletID      string      `json:"linked_wallet_id,omitempty"`
	TargetAmount        float64     `json:"target_amount,omitempty"`
	EOMSweepDestination string      `json:"eom_sweep_destination,omitempty"`
}

func NewSavingsWallet() Wallet {
	return Wallet{
		ID:    SavingsWalletID,
		Name:  "Savings",
		Scope: ScopeGlobalOnly,
	}
}

func (q Wallet) IsMonthly() bool {
	return q.Scope == ScopeMonthlyOnly
}

func (q Wallet) IsGlobal() bool {
	return q.Scope == ScopeGlobalOnly
}

func (q Wallet) HasGoal() bool {
	return q.IsGlobal() && q.TargetAmount > 0
}

func (q Wallet) GoalProgress(accumulated float64) float64 {
	if !q.HasGoal() {
		return 0
	}
	return (accumulated / q.TargetAmount) * 100
}

func (q Wallet) GetID() string {
	return q.ID
}

func (q *Wallet) ApplyDefaults(savingsID string) {
	if q.Scope == ScopeMonthlyOnly && q.EOMSweepDestination == "" {
		if q.LinkedWalletID != "" {
			q.EOMSweepDestination = q.LinkedWalletID
		} else {
			q.EOMSweepDestination = savingsID
		}
	}
}

func (q Wallet) Validate() error {
	if q.Name == "" {
		return errors.New("name is required")
	}

	switch q.Scope {
	case ScopeMonthlyOnly, ScopeGlobalOnly:

	default:
		return errors.New("scope must be 'Monthly' or 'Global'")
	}

	if q.Scope == ScopeMonthlyOnly && q.TargetAmount != 0 {
		return errors.New("target amount is not applicable to Monthly wallets")
	}
	if q.Scope == ScopeGlobalOnly && q.EOMSweepDestination != "" {
		return errors.New("EOM sweep destination is not applicable to Global wallets")
	}

	return nil
}

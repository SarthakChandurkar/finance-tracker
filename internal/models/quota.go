package models

import "errors"

// QuotaScope — A2.2. A quota is ALWAYS one or the other, never both.
// The "Both" creation mode (below) generates two separate Quota records
// that happen to be linked, rather than one record with a dual scope.
type QuotaScope string

const (
	ScopeMonthlyOnly QuotaScope = "Monthly Only"
	ScopeGlobalOnly  QuotaScope = "Global Only"
)

// CreationMode is only used at creation time (B2.1 quota creation form) to
// decide how many Quota records to generate and how to link them. It is not
// stored on the Quota itself afterwards.
type CreationMode string

const (
	CreateBoth        CreationMode = "Both"
	CreateMonthlyOnly CreationMode = "Monthly Only"
	CreateGlobalOnly  CreationMode = "Global Only"
)

// SavingsQuotaID is a fixed, well-known ID (not a randomly generated one)
// for the mandatory "Savings" quota described in A2.2. Using a constant,
// predictable ID means any code in the app can refer to "the Savings quota"
// without having to look it up by name first.
const SavingsQuotaID = "savings"

// Quota is a single budget entity — either Monthly Only or Global Only.
// See A2.2 for the full field-by-field rules.
type Quota struct {
	ID                  string     `json:"id"`
	Name                string     `json:"name"`
	Scope               QuotaScope `json:"scope"`
	LinkedQuotaID       string     `json:"linked_quota_id,omitempty"`
	MonthlyAllocation   float64    `json:"monthly_allocation,omitempty"`
	TargetAmount        float64    `json:"target_amount,omitempty"`
	EOMSweepDestination string     `json:"eom_sweep_destination,omitempty"`
	Archived            bool       `json:"archived"`
}

// NewSavingsQuota builds the one mandatory, non-deletable "Savings" quota
// (A2.2). Called once, the first time the app runs, to seed storage.
func NewSavingsQuota() Quota {
	return Quota{
		ID:    SavingsQuotaID,
		Name:  "Savings",
		Scope: ScopeGlobalOnly,
		// No TargetAmount set → unlimited bucket, per A2.4.
	}
}

// IsMonthly / IsGlobal are small convenience methods — used all over the
// domain layer instead of writing `q.Scope == models.ScopeMonthlyOnly`
// everywhere.
func (q Quota) IsMonthly() bool {
	return q.Scope == ScopeMonthlyOnly
}

func (q Quota) IsGlobal() bool {
	return q.Scope == ScopeGlobalOnly
}

// HasGoal reports whether this Global Only quota is a tracked Goal
// (Target Amount set) vs. an unlimited bucket (A2.4).
func (q Quota) HasGoal() bool {
	return q.IsGlobal() && q.TargetAmount > 0
}

// GoalProgress returns percent-complete (0-100) given the current
// accumulated balance. Reaching >100 is allowed (A2.4: "simply over-funded").
// Caller must check HasGoal() first — this returns 0 if there's no target,
// to avoid a divide-by-zero.
func (q Quota) GoalProgress(accumulated float64) float64 {
	if !q.HasGoal() {
		return 0
	}
	return (accumulated / q.TargetAmount) * 100
}

// ApplyDefaults fills in EOM Sweep Destination per the A2.2 rules:
// defaults to the Linked Quota if paired, otherwise "Savings".
// savingsID is passed in rather than hardcoded to SavingsQuotaID so this
// stays testable, but in practice callers will pass models.SavingsQuotaID.
func (q *Quota) ApplyDefaults(savingsID string) {
	if q.Scope == ScopeMonthlyOnly && q.EOMSweepDestination == "" {
		if q.LinkedQuotaID != "" {
			q.EOMSweepDestination = q.LinkedQuotaID
		} else {
			q.EOMSweepDestination = savingsID
		}
	}
}

// Validate enforces the "Rules / Applicability" column of the A2.2 table —
// mainly, that fields belonging to one scope don't leak into the other.
func (q Quota) Validate() error {
	if q.Name == "" {
		return errors.New("name is required")
	}

	switch q.Scope {
	case ScopeMonthlyOnly, ScopeGlobalOnly:
		// ok
	default:
		return errors.New("scope must be 'Monthly Only' or 'Global Only'")
	}

	if q.Scope == ScopeMonthlyOnly && q.MonthlyAllocation <= 0 {
		return errors.New("monthly allocation is required for Monthly Only quotas")
	}
	if q.Scope == ScopeGlobalOnly && q.MonthlyAllocation != 0 {
		return errors.New("monthly allocation is not applicable to Global Only quotas")
	}
	if q.Scope == ScopeMonthlyOnly && q.TargetAmount != 0 {
		return errors.New("target amount is not applicable to Monthly Only quotas")
	}
	if q.Scope == ScopeGlobalOnly && q.EOMSweepDestination != "" {
		return errors.New("EOM sweep destination is not applicable to Global Only quotas")
	}

	return nil
}
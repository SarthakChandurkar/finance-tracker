package domain

import (
	"fmt"
	"time"

	"financetracker/internal/models"
	"financetracker/internal/storage"
)

// QuotaService groups every operation related to quotas. It holds a
// reference to the Store so it can read/write data, but it never touches
// the JSON file directly — that's storage's job, not domain's.
type QuotaService struct {
	store *storage.Store
}

func NewQuotaService(store *storage.Store) *QuotaService {
	return &QuotaService{store: store}
}

// UpdateQuota updates the mutable fields of an existing quota.
func (s *QuotaService) UpdateQuota(quotaID string, name *string, targetAmount *float64, eomSweepDestination *string) (models.Quota, error) {
	var updated models.Quota
	err := s.store.Update(func(d *storage.Data) error {
		idx := -1
		for i, q := range d.Quotas {
			if q.ID == quotaID {
				idx = i
				updated = q
				break
			}
		}
		if idx == -1 {
			return fmt.Errorf("quota not found: %s", quotaID)
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
		updated.ApplyDefaults(models.SavingsQuotaID)
		if err := updated.Validate(); err != nil {
			return err
		}
		d.Quotas[idx] = updated
		return nil
	})
	return updated, err
}

// ArchiveQuota archives the quota and moves any remaining balance into Savings.
func monthlyBreakdownFromData(d *storage.Data, quotaID string, now time.Time) (MonthlyBreakdown, error) {
	result := MonthlyBreakdown{QuotaID: quotaID}

	var quota models.Quota
	var found bool
	for _, q := range d.Quotas {
		if q.ID == quotaID {
			quota = q
			found = true
			break
		}
	}
	if !found {
		return result, fmt.Errorf("quota not found: %s", quotaID)
	}
	if !quota.IsMonthly() {
		return result, fmt.Errorf("quota %s is not a Monthly Only quota", quotaID)
	}

	result.QuotaName = quota.Name
	for _, tx := range d.Transactions {

		// fmt.Println(tx.Date.Year(), now.Year(), tx.Date.Month(), now.Month())

		if tx.Date.Year() != now.Year() || tx.Date.Month() != now.Month() {
			continue
		}
		switch tx.Type {
		case models.Debit:
			if tx.SourceQuotaID == quotaID {
				result.Debited += tx.Amount
			}
		case models.Credit, models.Salary, models.LoanReceived:
			if tx.DestinationQuotaID == quotaID {
				result.CreditsIn += tx.Amount
			}
		case models.SelfTransfer:
			if tx.DestinationQuotaID == quotaID {
				result.TransfersIn += tx.Amount
			}
			if tx.SourceQuotaID == quotaID {
				result.TransfersOut += tx.Amount
			}
		case models.InterQuotaLoan:
			if tx.DestinationQuotaID == quotaID {
				result.LoansIn += tx.Amount
			}
			if tx.SourceQuotaID == quotaID {
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

func globalBreakdownFromData(d *storage.Data, quotaID string) (GlobalBreakdown, error) {
	result := GlobalBreakdown{QuotaID: quotaID}

	var quota models.Quota
	var found bool
	for _, q := range d.Quotas {
		if q.ID == quotaID {
			quota = q
			found = true
			break
		}
	}
	if !found {
		return result, fmt.Errorf("quota not found: %s", quotaID)
	}
	if !quota.IsGlobal() {
		return result, fmt.Errorf("quota %s is not a Global Only quota", quotaID)
	}

	result.QuotaName = quota.Name
	for _, tx := range d.Transactions {
		switch tx.Type {
		case models.Debit:
			if tx.SourceQuotaID == quotaID {
				result.Accumulated -= tx.Amount
			}
		case models.Credit, models.Salary, models.LoanReceived:
			if tx.DestinationQuotaID == quotaID {
				result.Accumulated += tx.Amount
			}
		case models.SelfTransfer:
			if tx.DestinationQuotaID == quotaID {
				result.Accumulated += tx.Amount
			}
			if tx.SourceQuotaID == quotaID {
				result.Accumulated -= tx.Amount
			}
		case models.InterQuotaLoan:
			// See the matching case in balances.go's globalAccumulated for
			// why only the source (lending) side applies here.
			if tx.SourceQuotaID == quotaID {
				result.Accumulated -= tx.Amount
			}
		}
	}

	result.HasGoal = quota.HasGoal()
	result.TargetAmount = quota.TargetAmount
	if result.HasGoal {
		result.PercentComplete = quota.GoalProgress(result.Accumulated)
	}
	return result, nil
}

func (s *QuotaService) AllMonthlyBreakdowns(now time.Time) ([]MonthlyBreakdown, error) {
	var results []MonthlyBreakdown
	s.store.View(func(d storage.Data) {
		for _, q := range d.Quotas {
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

func (s *QuotaService) AllGlobalBreakdowns() ([]GlobalBreakdown, error) {
	var results []GlobalBreakdown
	s.store.View(func(d storage.Data) {
		for _, q := range d.Quotas {
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

// DeleteQuota permanently removes a quota after sweeping any remaining
// balance into the mandatory Savings quota. Unlike the old "archive" flag,
// the quota record is actually removed from storage here — there's no
// hidden archived copy left behind, and no way to "un-delete" it. If the
// quota was one half of a linked pair, the other half is unlinked (and,
// if it was relying on this quota as its EOM Sweep Destination, that
// falls back to Savings so it doesn't keep pointing at a deleted quota).
func (s *QuotaService) DeleteQuota(quotaID string) (models.Quota, error) {
	if quotaID == models.SavingsQuotaID {
		return models.Quota{}, fmt.Errorf("the mandatory Savings quota cannot be deleted")
	}

	var deleted models.Quota
	err := s.store.Update(func(d *storage.Data) error {
		idx := -1
		for i, q := range d.Quotas {
			if q.ID == quotaID {
				idx = i
				deleted = q
				break
			}
		}
		if idx == -1 {
			return fmt.Errorf("quota not found: %s", quotaID)
		}

		if hasOutstandingInterQuotaLoan(d, quotaID) {
			return fmt.Errorf("cannot delete this quota: it has an outstanding Inter-Quota Loan attached to it — settle it first")
		}

		balance, err := availableBalance(d, quotaID, time.Now())
		if err != nil {
			return err
		}
		if balance > 0 {
			d.Transactions = append(d.Transactions, models.Transaction{
				ID:                 newID(),
				Type:               models.SelfTransfer,
				Amount:             balance,
				SourceQuotaID:      quotaID,
				DestinationQuotaID: models.SavingsQuotaID,
				Date:               time.Now(),
				Details:            "system:quota-deletion-transfer",
			})
		}

		d.Quotas = append(d.Quotas[:idx], d.Quotas[idx+1:]...)

		if deleted.LinkedQuotaID != "" {
			for j, other := range d.Quotas {
				if other.ID == deleted.LinkedQuotaID {
					other.LinkedQuotaID = ""
					if other.EOMSweepDestination == quotaID {
						other.EOMSweepDestination = models.SavingsQuotaID
					}
					d.Quotas[j] = other
					break
				}
			}
		}
		return nil
	})
	return deleted, err
}

// CreateQuota implements the three creation modes from A2.2. For "Both",
// it builds two linked entities in one call, per the "Entity Model Note"
// at the top of A2: a quota is always single-scope, and "Both" just means
// "make two of them and link them."
//
// It returns a Go SLICE ([]models.Quota) because "Both" mode produces two
// records but the other two modes produce one — a slice lets the caller
// handle all three cases the same way (loop over however many came back).
func (s *QuotaService) CreateQuota(mode models.CreationMode, name string, targetAmount float64, eomSweepDestination string) ([]models.Quota, error) {
	switch mode {

	case models.CreateBoth:
		monthlyID := newID()
		globalID := newID()

		monthly := models.Quota{
			ID:            monthlyID,
			Name:          name,
			Scope:         models.ScopeMonthlyOnly,
			LinkedQuotaID: globalID,
		}
		global := models.Quota{
			ID:            globalID,
			Name:          name,
			Scope:         models.ScopeGlobalOnly,
			LinkedQuotaID: monthlyID,
			TargetAmount:  targetAmount,
		}
		// LinkedQuotaID is already set on `monthly`, so ApplyDefaults will
		// set its EOMSweepDestination to globalID automatically (A2.2:
		// "Monthly entity's EOM Sweep Destination auto-defaults to its
		// linked Global entity").
		monthly.ApplyDefaults(models.SavingsQuotaID)

		if err := monthly.Validate(); err != nil {
			return nil, fmt.Errorf("monthly half: %w", err)
		}
		if err := global.Validate(); err != nil {
			return nil, fmt.Errorf("global half: %w", err)
		}

		err := s.store.Update(func(d *storage.Data) error {
			d.Quotas = append(d.Quotas, monthly, global)
			return nil
		})
		if err != nil {
			return nil, err
		}
		return []models.Quota{monthly, global}, nil

	case models.CreateMonthlyOnly:

		q := models.Quota{
			ID:                  newID(),
			Name:                name,
			Scope:               models.ScopeMonthlyOnly,
			EOMSweepDestination: eomSweepDestination,
		}
		// No LinkedQuotaID here, so ApplyDefaults falls back to "Savings"
		// if the caller didn't set an explicit EOMSweepDestination — per
		// A2.2's "Monthly Only" creation mode rule.
		q.ApplyDefaults(models.SavingsQuotaID)

		if err := q.Validate(); err != nil {
			return nil, err
		}
		err := s.store.Update(func(d *storage.Data) error {
			d.Quotas = append(d.Quotas, q)
			return nil
		})
		if err != nil {
			return nil, err
		}
		return []models.Quota{q}, nil

	case models.CreateGlobalOnly:
		q := models.Quota{
			ID:           newID(),
			Name:         name,
			Scope:        models.ScopeGlobalOnly,
			TargetAmount: targetAmount,
		}
		if err := q.Validate(); err != nil {
			return nil, err
		}
		err := s.store.Update(func(d *storage.Data) error {
			d.Quotas = append(d.Quotas, q)
			return nil
		})
		if err != nil {
			return nil, err
		}
		return []models.Quota{q}, nil

	default:
		return nil, fmt.Errorf("unknown creation mode: %q", mode)
	}
}

// MonthlyBreakdown is the computed A2.3 view for one Monthly Only quota.
// Nothing here is stored — it's entirely derived from the transaction
// list, filtered to the given month, every time it's requested.
type MonthlyBreakdown struct {
	QuotaID          string  `json:"quota_id"`
	QuotaName        string  `json:"quota_name,omitempty"`
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

// MonthlyBreakdown computes A2.3's formula:
//
//	Available Balance = Allocated + Self Transfers In + Loans In
//	                     − Debited − Loans Out − Self Transfers Out
//
// `now` is passed in (rather than calling time.Now() internally) so this
// function is easy to test — a test can pass any month it likes and get a
// predictable answer.
func (s *QuotaService) MonthlyBreakdown(quotaID string, now time.Time) (MonthlyBreakdown, error) {
	result := MonthlyBreakdown{QuotaID: quotaID}

	var quota models.Quota
	var found bool

	s.store.View(func(d storage.Data) {
		for _, q := range d.Quotas {
			if q.ID == quotaID {
				quota = q
				found = true
				break
			}
		}
		if !found {
			return
		}
		result.QuotaName = quota.Name

		for _, tx := range d.Transactions {
			if tx.Date.Year() != now.Year() || tx.Date.Month() != now.Month() {
				continue // outside the current calendar month
			}
			switch tx.Type {
			case models.Debit:
				if tx.SourceQuotaID == quotaID {
					result.Debited += tx.Amount
				}
			case models.Credit, models.Salary, models.LoanReceived:
				if tx.DestinationQuotaID == quotaID {
					result.CreditsIn += tx.Amount
				}
			case models.SelfTransfer:
				if tx.DestinationQuotaID == quotaID {
					result.TransfersIn += tx.Amount
				}
				if tx.SourceQuotaID == quotaID {
					result.TransfersOut += tx.Amount
				}
			case models.InterQuotaLoan:
				if tx.DestinationQuotaID == quotaID {
					result.LoansIn += tx.Amount
				}
				if tx.SourceQuotaID == quotaID {
					result.LoansOut += tx.Amount
				}
			}
		}
	})

	if !found {
		return result, fmt.Errorf("quota not found: %s", quotaID)
	}
	if !quota.IsMonthly() {
		return result, fmt.Errorf("quota %s is not a Monthly Only quota", quotaID)
	}

	result.AvailableBalance = result.Allocated + result.TransfersIn + result.LoansIn + result.CreditsIn -
		result.Debited - result.LoansOut - result.TransfersOut

	inflow := result.Allocated + result.TransfersIn + result.LoansIn + result.CreditsIn
	if inflow > 0 {
		result.DebitPercent = (result.Debited / inflow) * 100
	}

	return result, nil
}

// GlobalBreakdown is the computed A2.4 view for one Global Only quota.
type GlobalBreakdown struct {
	QuotaID         string  `json:"quota_id"`
	QuotaName       string  `json:"quota_name,omitempty"`
	Accumulated     float64 `json:"accumulated"`
	HasGoal         bool    `json:"has_goal"`
	TargetAmount    float64 `json:"target_amount"`
	PercentComplete float64 `json:"percent_complete"`
}

// GlobalBreakdown sums EVERY transaction ever recorded against this quota
// (no month filter — "no date restriction: all history" is the whole
// point of a Global entity per A2.4), then folds in the Goal math from
// A2.4/Quota.GoalProgress.
func (s *QuotaService) GlobalBreakdown(quotaID string) (GlobalBreakdown, error) {
	result := GlobalBreakdown{QuotaID: quotaID}

	var quota models.Quota
	var found bool

	s.store.View(func(d storage.Data) {
		for _, q := range d.Quotas {
			if q.ID == quotaID {
				quota = q
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
				if tx.SourceQuotaID == quotaID {
					result.Accumulated -= tx.Amount
				}
			case models.Credit, models.Salary, models.LoanReceived:
				if tx.DestinationQuotaID == quotaID {
					result.Accumulated += tx.Amount
				}
			case models.SelfTransfer:
				if tx.DestinationQuotaID == quotaID {
					result.Accumulated += tx.Amount
				}
				if tx.SourceQuotaID == quotaID {
					result.Accumulated -= tx.Amount
				}
			case models.InterQuotaLoan:
				// Per A2.6, a Global Only quota can only ever be the LENDER
				// in an Inter-Quota Loan (loans only ever lend INTO a
				// Monthly quota — see validateInterQuotaLoan), so only the
				// source (lending) side applies here. Lending money out
				// reduces the lender's own accumulated balance; it's repaid
				// later via an automatic SelfTransfer (see RunMonthStart),
				// which the SelfTransfer case above already credits back.
				if tx.SourceQuotaID == quotaID {
					result.Accumulated -= tx.Amount
				}
			}
		}
	})

	if !found {
		return result, fmt.Errorf("quota not found: %s", quotaID)
	}
	result.QuotaName = quota.Name
	if !quota.IsGlobal() {
		return result, fmt.Errorf("quota %s is not a Global Only quota", quotaID)
	}

	result.HasGoal = quota.HasGoal()
	result.TargetAmount = quota.TargetAmount
	if result.HasGoal {
		result.PercentComplete = quota.GoalProgress(result.Accumulated)
	}

	return result, nil
}

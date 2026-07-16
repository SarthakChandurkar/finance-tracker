package models

import "errors"

// SettledCategoryID mirrors SavingsQuotaID from quota.go — a fixed,
// predictable ID for the one mandatory category, so the rest of the app
// can always find it without a name lookup (A4.2).
const SettledCategoryID = "settled"

// SettledCategoryName is the display name for the mandatory category.
const SettledCategoryName = "Settled"

// Category — A4.1. Deliberately much simpler than Quota: "no Scope, no
// linked entities (unlike quotas)" per the requirements doc. Just an ID
// and a Name.
type Category struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// NewSettledCategory builds the one mandatory, non-deletable "Settled"
// category (A4.2) — the fallback destination for orphaned transactions
// when their original category gets deleted. Seeded once at app startup,
// same pattern as NewSavingsQuota in quota.go.
func NewSettledCategory() Category {
	return Category{
		ID:   SettledCategoryID,
		Name: SettledCategoryName,
	}
}

// Validate just checks the Name isn't empty — there's nothing else to
// validate here, which is the point: A4.1 explicitly calls out that a
// Category record carries none of the scope/linking complexity a Quota
// does.
func (c Category) Validate() error {
	if c.Name == "" {
		return errors.New("name is required")
	}
	return nil
}
package domain

import (
	"path/filepath"
	"testing"
	"time"

	"financetracker/internal/models"
	"financetracker/internal/storage"
)

func newTestStore(t *testing.T) *storage.Store {
	tt := t.TempDir()
	file := filepath.Join(tt, "data.json")
	store := storage.NewStore(file)
	if err := store.Load(); err != nil {
		t.Fatalf("failed to load store: %v", err)
	}
	return store
}

func TestQuotaService_AllBreakdowns(t *testing.T) {
	store := newTestStore(t)
	quotaSvc := NewQuotaService(store)
	txSvc := NewTransactionService(store)

	// Create one monthly quota and one global quota.
	monthlyQuotas, err := quotaSvc.CreateQuota(models.CreateMonthlyOnly, "Food", 1000, 0, models.SavingsQuotaID)
	if err != nil {
		t.Fatalf("CreateQuota monthly: %v", err)
	}
	globalQuotas, err := quotaSvc.CreateQuota(models.CreateGlobalOnly, "Vacation", 0, 5000, "")
	if err != nil {
		t.Fatalf("CreateQuota global: %v", err)
	}

	monthlyQuotaID := monthlyQuotas[0].ID
	globalQuotaID := globalQuotas[0].ID

	now := time.Now()

	// Fund the monthly quota with a credit so it can be spent.
	if _, err := txSvc.RecordTransaction(models.Transaction{
		Type:               models.Credit,
		Amount:             1000,
		DestinationQuotaID: monthlyQuotaID,
		Date:               now,
	}); err != nil {
		t.Fatalf("RecordTransaction monthly credit: %v", err)
	}

	// Fund the global quota with income so it can transfer money.
	if _, err := txSvc.RecordTransaction(models.Transaction{
		Type:               models.Credit,
		Amount:             400,
		DestinationQuotaID: globalQuotaID,
		Date:               now,
	}); err != nil {
		t.Fatalf("RecordTransaction credit: %v", err)
	}

	// Add a debit from the monthly quota.
	if _, err := txSvc.RecordTransaction(models.Transaction{
		Type:          models.Debit,
		Amount:        200,
		SourceQuotaID: monthlyQuotaID,
		Category:      "Food",
		Date:          now,
	}); err != nil {
		t.Fatalf("RecordTransaction debit: %v", err)
	}

	// Add a self transfer from the global quota into the monthly quota.
	if _, err := txSvc.RecordTransaction(models.Transaction{
		Type:               models.SelfTransfer,
		Amount:             150,
		SourceQuotaID:      globalQuotaID,
		DestinationQuotaID: monthlyQuotaID,
		Date:               now,
	}); err != nil {
		t.Fatalf("RecordTransaction self transfer: %v", err)
	}

	monthly, err := quotaSvc.AllMonthlyBreakdowns(now)
	if err != nil {
		t.Fatalf("AllMonthlyBreakdowns: %v", err)
	}
	if len(monthly) != 1 {
		t.Fatalf("expected 1 monthly breakdown, got %d", len(monthly))
	}
	if monthly[0].QuotaID != monthlyQuotaID {
		t.Fatalf("expected monthly quota %s, got %s", monthlyQuotaID, monthly[0].QuotaID)
	}
	if monthly[0].Debited != 200 {
		t.Fatalf("expected monthly debited 200, got %v", monthly[0].Debited)
	}
	if monthly[0].TransfersIn != 150 {
		t.Fatalf("expected transfers in 150, got %v", monthly[0].TransfersIn)
	}

	global, err := quotaSvc.AllGlobalBreakdowns()
	if err != nil {
		t.Fatalf("AllGlobalBreakdowns: %v", err)
	}
	if len(global) != 2 {
		t.Fatalf("expected 2 global breakdowns (Savings + Vacation), got %d", len(global))
	}
	var found bool
	for _, g := range global {
		if g.QuotaID != globalQuotaID {
			continue
		}
		found = true
		if g.Accumulated != 250 {
			t.Fatalf("expected global accumulated 250, got %v", g.Accumulated)
		}
	}
	if !found {
		t.Fatalf("expected global breakdown for quota %s", globalQuotaID)
	}
}

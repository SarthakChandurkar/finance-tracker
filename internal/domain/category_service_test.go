package domain

import (
	"path/filepath"
	"testing"
	"time"

	"financetracker/internal/models"
	"financetracker/internal/storage"
)

func newCategoryTestStore(t *testing.T) *storage.Store {
	tt := t.TempDir()
	file := filepath.Join(tt, "data.json")
	store := storage.NewStore(file)
	if err := store.Load(); err != nil {
		t.Fatalf("failed to load store: %v", err)
	}
	return store
}

func TestCategoryService_Totals(t *testing.T) {
	store := newCategoryTestStore(t)
	catSvc := NewCategoryService(store)
	txSvc := NewTransactionService(store)

	if _, err := catSvc.CreateCategory("Groceries"); err != nil {
		t.Fatalf("CreateCategory: %v", err)
	}

	foodCategory := "Groceries"
	date := time.Now()

	if _, err := txSvc.RecordTransaction(models.Transaction{
		Type:               models.Credit,
		Amount:             200,
		DestinationQuotaID: models.SavingsQuotaID,
		Date:               date,
	}); err != nil {
		t.Fatalf("RecordTransaction funding savings: %v", err)
	}

	if _, err := txSvc.RecordTransaction(models.Transaction{
		Type:          models.Debit,
		Amount:        120,
		SourceQuotaID: models.SavingsQuotaID,
		Category:      foodCategory,
		Date:          date,
	}); err != nil {
		t.Fatalf("RecordTransaction debit: %v", err)
	}

	if _, err := txSvc.RecordTransaction(models.Transaction{
		Type:          models.Debit,
		Amount:        80,
		SourceQuotaID: models.SavingsQuotaID,
		Category:      foodCategory,
		Date:          date.AddDate(0, -1, 0),
	}); err != nil {
		t.Fatalf("RecordTransaction historical debit: %v", err)
	}

	monthly, err := catSvc.MonthlyTotals(date)
	if err != nil {
		t.Fatalf("MonthlyTotals: %v", err)
	}
	if len(monthly) == 0 {
		t.Fatal("expected monthly totals to include categories")
	}
	found := false
	for _, total := range monthly {
		if total.CategoryName == foodCategory {
			found = true
			if total.Total != 120 {
				t.Fatalf("expected monthly total 120, got %v", total.Total)
			}
		}
	}
	if !found {
		t.Fatal("expected Groceries category in monthly totals")
	}

	global, err := catSvc.GlobalTotals()
	if err != nil {
		t.Fatalf("GlobalTotals: %v", err)
	}
	found = false
	for _, total := range global {
		if total.CategoryName == foodCategory {
			found = true
			if total.Total != 200 {
				t.Fatalf("expected global total 200, got %v", total.Total)
			}
		}
	}
	if !found {
		t.Fatal("expected Groceries category in global totals")
	}
}

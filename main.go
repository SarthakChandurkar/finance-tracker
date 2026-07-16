package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"financetracker/internal/domain"
	"financetracker/internal/models"
	"financetracker/internal/storage"
)

type apiServer struct {
	store           *storage.Store
	txService       *domain.TransactionService
	quotaService    *domain.QuotaService
	categoryService *domain.CategoryService
	loanService     *domain.LoanService
	schedule        *domain.ScheduleService
}

func main() {
	dataFile := os.Getenv("DATA_FILE")
	if dataFile == "" {
		dataFile = "data.json"
	}

	store := storage.NewStore(dataFile)
	if err := store.Load(); err != nil {
		log.Fatalf("failed to load data file: %v", err)
	}

	server := &apiServer{
		store:           store,
		txService:       domain.NewTransactionService(store),
		quotaService:    domain.NewQuotaService(store),
		categoryService: domain.NewCategoryService(store),
		loanService:     domain.NewLoanService(store),
		schedule:        domain.NewScheduleService(store),
	}

	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}

	log.Printf("starting finance tracker backend on %s (data file=%s)", addr, dataFile)
	log.Fatal(http.ListenAndServe(addr, server.routes()))
}

func (s *apiServer) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/transactions", s.handleTransactions)
	mux.HandleFunc("/api/transactions/", s.handleTransactionByID)
	mux.HandleFunc("/api/quotas", s.handleQuotas)
	mux.HandleFunc("/api/quotas/breakdowns", s.handleQuotaBreakdowns)
	mux.HandleFunc("/api/quotas/", s.handleQuotaByID)
	mux.HandleFunc("/api/categories", s.handleCategories)
	mux.HandleFunc("/api/categories/totals", s.handleCategoryTotals)
	mux.HandleFunc("/api/categories/", s.handleCategoryByID)
	mux.HandleFunc("/api/categories/recategorize", s.handleCategoryRecategorize)
	mux.HandleFunc("/api/funding-remediation", s.handleFundingRemediation)
	mux.HandleFunc("/api/export", s.handleExport)
	mux.HandleFunc("/api/import", s.handleImport)
	mux.HandleFunc("/api/loan-ledger", s.handleLoanLedger)
	mux.HandleFunc("/api/schedule/start", s.handleScheduleStart)
	mux.HandleFunc("/api/schedule/end", s.handleScheduleEnd)
	mux.HandleFunc("/api/funding-options", s.handleFundingOptions)
	mux.Handle("/", http.FileServer(http.Dir("ui")))
	return mux
}

func (s *apiServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *apiServer) handleTransactions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.store.View(func(d storage.Data) {
			writeJSON(w, http.StatusOK, d.Transactions)
		})
	case http.MethodPost:
		var req models.Transaction
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
			return
		}
		saved, err := s.txService.RecordTransaction(req)
		if err != nil {
			writeAPIError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, saved)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *apiServer) handleTransactionByID(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, "/api/transactions/") {
		http.NotFound(w, r)
		return
	}
	id := path.Base(r.URL.Path)
	switch r.Method {
	case http.MethodPut:
		var req models.Transaction
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
			return
		}
		saved, err := s.txService.EditTransaction(id, req)
		if err != nil {
			writeAPIError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, saved)
	case http.MethodDelete:
		if err := s.txService.DeleteTransaction(id); err != nil {
			writeAPIError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *apiServer) handleQuotas(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.store.View(func(d storage.Data) {
			writeJSON(w, http.StatusOK, d.Quotas)
		})
	case http.MethodPost:
		var req struct {
			Mode                models.CreationMode `json:"mode"`
			Name                string              `json:"name"`
			MonthlyAllocation   float64             `json:"monthly_allocation,omitempty"`
			TargetAmount        float64             `json:"target_amount,omitempty"`
			EOMSweepDestination string              `json:"eom_sweep_destination,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
			return
		}
		created, err := s.quotaService.CreateQuota(req.Mode, req.Name, req.MonthlyAllocation, req.TargetAmount, req.EOMSweepDestination)
		if err != nil {
			writeAPIError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, created)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *apiServer) handleQuotaBreakdowns(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	scope := r.URL.Query().Get("scope")
	now := time.Now()

	switch scope {
	case "", "all":
		monthly, err := s.quotaService.AllMonthlyBreakdowns(now)
		if err != nil {
			writeAPIError(w, err)
			return
		}
		global, err := s.quotaService.AllGlobalBreakdowns()
		if err != nil {
			writeAPIError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"monthly": monthly, "global": global})
	case "monthly":
		monthly, err := s.quotaService.AllMonthlyBreakdowns(now)
		if err != nil {
			writeAPIError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, monthly)
	case "global":
		global, err := s.quotaService.AllGlobalBreakdowns()
		if err != nil {
			writeAPIError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, global)
	default:
		http.Error(w, "invalid scope value", http.StatusBadRequest)
	}
}

func (s *apiServer) handleQuotaByID(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, "/api/quotas/") {
		http.NotFound(w, r)
		return
	}
	id := path.Base(r.URL.Path)
	switch r.Method {
	case http.MethodPut:
		var req struct {
			Name                *string  `json:"name,omitempty"`
			MonthlyAllocation   *float64 `json:"monthly_allocation,omitempty"`
			TargetAmount        *float64 `json:"target_amount,omitempty"`
			EOMSweepDestination *string  `json:"eom_sweep_destination,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
			return
		}
		saved, err := s.quotaService.UpdateQuota(id, req.Name, req.MonthlyAllocation, req.TargetAmount, req.EOMSweepDestination)
		if err != nil {
			writeAPIError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, saved)
	case http.MethodDelete:
		saved, err := s.quotaService.ArchiveQuota(id)
		if err != nil {
			writeAPIError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, saved)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *apiServer) handleCategories(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.store.View(func(d storage.Data) {
			writeJSON(w, http.StatusOK, d.Categories)
		})
	case http.MethodPost:
		var req struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
			return
		}
		category, err := s.categoryService.CreateCategory(req.Name)
		if err != nil {
			writeAPIError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, category)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *apiServer) handleCategoryTotals(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	scope := r.URL.Query().Get("scope")
	now := time.Now()

	switch scope {
	case "", "all":
		monthly, err := s.categoryService.MonthlyTotals(now)
		if err != nil {
			writeAPIError(w, err)
			return
		}
		global, err := s.categoryService.GlobalTotals()
		if err != nil {
			writeAPIError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"monthly": monthly, "global": global})
	case "monthly":
		monthly, err := s.categoryService.MonthlyTotals(now)
		if err != nil {
			writeAPIError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, monthly)
	case "global":
		global, err := s.categoryService.GlobalTotals()
		if err != nil {
			writeAPIError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, global)
	default:
		http.Error(w, "invalid scope value", http.StatusBadRequest)
	}
}

func (s *apiServer) handleCategoryByID(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, "/api/categories/") {
		http.NotFound(w, r)
		return
	}
	id := path.Base(r.URL.Path)
	switch r.Method {
	case http.MethodPut:
		var req struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
			return
		}
		updated, err := s.categoryService.RenameCategory(id, req.Name)
		if err != nil {
			writeAPIError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, updated)
	case http.MethodDelete:
		deleted, err := s.categoryService.DeleteCategory(id)
		if err != nil {
			writeAPIError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, deleted)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *apiServer) handleExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.store.View(func(d storage.Data) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", "attachment; filename=finance-tracker-export.json")
		if err := json.NewEncoder(w).Encode(d); err != nil {
			http.Error(w, fmt.Sprintf("failed to encode export data: %v", err), http.StatusInternalServerError)
		}
	})
}

func (s *apiServer) handleCategoryRecategorize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		SourceCategoryID string `json:"source_category_id"`
		DestCategoryID   string `json:"dest_category_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
		return
	}
	if err := s.categoryService.Recategorize(req.SourceCategoryID, req.DestCategoryID); err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "recategorized"})
}

func (s *apiServer) handleImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var imported storage.Data
	if err := json.NewDecoder(r.Body).Decode(&imported); err != nil {
		http.Error(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
		return
	}
	if err := validateImportData(imported); err != nil {
		writeAPIError(w, err)
		return
	}
	if err := s.store.Update(func(d *storage.Data) error {
		*d = imported
		return nil
	}); err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "imported"})
}

func (s *apiServer) handleScheduleStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	now := time.Now()
	if err := s.schedule.RunMonthStart(now); err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "month-start completed"})
}

func (s *apiServer) handleScheduleEnd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	now := time.Now()
	if err := s.schedule.RunMonthEndSweep(now); err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "month-end sweep completed"})
}

func (s *apiServer) handleFundingOptions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	quotaID := r.URL.Query().Get("quota_id")
	amountParam := r.URL.Query().Get("amount")
	if quotaID == "" || amountParam == "" {
		http.Error(w, "quota_id and amount are required", http.StatusBadRequest)
		return
	}
	amount, err := strconv.ParseFloat(amountParam, 64)
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid amount: %v", err), http.StatusBadRequest)
		return
	}
	options, err := s.txService.EligibleFundingOptions(quotaID, amount, time.Now())
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, options)
}

func (s *apiServer) handleFundingRemediation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Mechanism         models.TransactionType   `json:"mechanism"`
		SourceQuotaID     string                   `json:"source_quota_id"`
		TargetQuotaID     string                   `json:"target_quota_id"`
		Amount            float64                  `json:"amount"`
		Category          string                   `json:"category,omitempty"`
		Counterparty      string                   `json:"counterparty,omitempty"`
		PaymentMode       models.PaymentMode       `json:"payment_mode,omitempty"`
		PaymentInstrument models.PaymentInstrument `json:"payment_instrument,omitempty"`
		Date              time.Time                `json:"date,omitempty"`
		Details           string                   `json:"details,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
		return
	}
	if req.Mechanism != models.SelfTransfer && req.Mechanism != models.InterQuotaLoan {
		http.Error(w, "mechanism must be Self Transfer or Inter-Quota Loan", http.StatusBadRequest)
		return
	}
	funding := domain.FundingOption{
		Mechanism:     req.Mechanism,
		SourceQuotaID: req.SourceQuotaID,
	}
	debit := models.Transaction{
		Type:              models.Debit,
		Amount:            req.Amount,
		SourceQuotaID:     req.TargetQuotaID,
		Category:          req.Category,
		Counterparty:      req.Counterparty,
		PaymentMode:       req.PaymentMode,
		PaymentInstrument: req.PaymentInstrument,
		Date:              req.Date,
		Details:           req.Details,
	}
	fundingTx, debitTx, err := s.txService.FundAndCreateDebit(funding, debit)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]models.Transaction{"funding": fundingTx, "debit": debitTx})
}

func (s *apiServer) handleLoanLedger(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	entries, err := s.loanService.CounterpartyLedger()
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, entries)
}

func validateImportData(data storage.Data) error {
	hasSavings := false
	hasSettled := false
	for _, q := range data.Quotas {
		if q.ID == models.SavingsQuotaID {
			hasSavings = true
		}
	}
	for _, c := range data.Categories {
		if c.ID == models.SettledCategoryID {
			hasSettled = true
		}
	}
	if !hasSavings {
		return fmt.Errorf("imported data must include the mandatory Savings quota")
	}
	if !hasSettled {
		return fmt.Errorf("imported data must include the mandatory Settled category")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeAPIError(w http.ResponseWriter, err error) {
	if err == nil {
		http.Error(w, "unexpected nil error", http.StatusInternalServerError)
		return
	}
	if insuff, ok := err.(*domain.InsufficientFundsError); ok {
		writeJSON(w, http.StatusUnprocessableEntity, insuff)
		return
	}
	http.Error(w, err.Error(), http.StatusBadRequest)
}

func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

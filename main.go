package main

import (
	"context"
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

	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"

	"financetracker/internal/domain"
	"financetracker/internal/models"
	"financetracker/internal/remote"
	"financetracker/internal/storage"
)

var ctx = context.Background()

var rdb *redis.Client

func init() {
	err := godotenv.Load()
	if err != nil {
		log.Println("no .env file found, relying on system env vars")
	}
	rdb = redis.NewClient(storage.NewRedisOptions())

	pong, err := rdb.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("could not connect to redis: %v", err)
	}
	fmt.Println("Connected to Redis DB:", pong)
}

type apiServer struct {
	store           *storage.Store
	txService       *domain.TransactionService
	walletService   *domain.WalletService
	categoryService *domain.CategoryService
	loanService     *domain.LoanService
	schedule        *domain.ScheduleService
	tasks           *remote.TodoClient
	authService     *domain.AuthService
}

// sessionTTLSeconds mirrors the session TTL configured in main() and is
// used as the session cookie's MaxAge.
var sessionTTLSeconds int

func main() {

	setupLogging()

	// ************************* Server Initialization ********************************************
	store := storage.NewStore(rdb)
	if err := store.Load(); err != nil {
		log.Fatalf("failed to load data from redis: %v", err)
	}

	todoServerURL := os.Getenv("TODO_SERVER_URL")
	if todoServerURL == "" {
		todoServerURL = remote.DefaultTodoServerURL
	}

	sessionTTL := 20 * time.Minute
	if raw := os.Getenv("SESSION_TTL_MINUTES"); raw != "" {
		if mins, err := strconv.Atoi(raw); err == nil && mins > 0 {
			sessionTTL = time.Duration(mins) * time.Minute
		} else {
			log.Printf("invalid SESSION_TTL_MINUTES=%q, using default of %s", raw, sessionTTL)
		}
	}
	sessionTTLSeconds = int(sessionTTL.Seconds())
	sessionStore := storage.NewSessionStore(rdb, sessionTTL)

	server := &apiServer{
		store:           store,
		txService:       domain.NewTransactionService(store),
		walletService:   domain.NewWalletService(store),
		categoryService: domain.NewCategoryService(store),
		loanService:     domain.NewLoanService(store),
		schedule:        domain.NewScheduleService(store),
		tasks:           remote.NewTodoClient(todoServerURL, remote.TodoServerTimeout),
		authService:     domain.NewAuthService(store, sessionStore),
	}

	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":2911"
	}

	baseHandler := server.routes()

	srv := &http.Server{
		Addr:    addr,
		Handler: baseHandler,
	}

	srv.Protocols = new(http.Protocols)
	srv.Protocols.SetHTTP1(true)
	srv.Protocols.SetUnencryptedHTTP2(true)

	log.Println("server starting at PORT", addr)
	log.Fatal(srv.ListenAndServe())
}

// **********************************MiddleWare Methods****************************************88

func (s *apiServer) reloadMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := s.store.Load(); err != nil {
			log.Printf("reloadMiddleware: failed to reload store from redis: %v", err)
		}
		next.ServeHTTP(w, r)
	})
}

// *************************************Route Handler Method*********************************************

func (s *apiServer) routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/register", s.handleRegister)
	mux.HandleFunc("/api/login", s.handleLogin)
	mux.HandleFunc("/api/logout", s.handleLogout)

	mux.HandleFunc("/api/status", s.handleStatus)

	mux.HandleFunc("/api/wallets", s.handleWallets)
	mux.HandleFunc("/api/wallets/breakdowns", s.handleWalletBreakdowns)
	mux.HandleFunc("/api/wallets/", s.handleWalletByID)

	mux.HandleFunc("/api/transactions", s.handleTransactions)
	mux.HandleFunc("/api/transactions/", s.handleTransactionByID)

	mux.HandleFunc("/api/categories", s.handleCategories)
	mux.HandleFunc("/api/categories/totals", s.handleCategoryTotals)
	mux.HandleFunc("/api/categories/", s.handleCategoryByID)
	mux.HandleFunc("/api/categories/recategorize", s.handleCategoryRecategorize)

	mux.HandleFunc("/api/funding-remediation", s.handleFundingRemediation)

	mux.HandleFunc("/api/loan-ledger", s.handleLoanLedger)

	mux.HandleFunc("/api/inter-wallet-loans", s.handleInterWalletLoans)
	mux.HandleFunc("/api/inter-wallet-loans/settle", s.handleSettleInterWalletLoan)

	mux.HandleFunc("/api/schedule/end", s.handleScheduleEnd)
	mux.HandleFunc("/api/funding-options", s.handleFundingOptions)

	mux.HandleFunc("/api/tasks", s.handleTasks)
	mux.HandleFunc("/api/tasks/", s.handleTaskbyID)

	mux.Handle("/", http.FileServer(http.Dir("ui")))

	return loggingMiddleware(s.reloadMiddleware(s.sessionMiddleware(mux)))
}

// Response Writing Functions

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

// ******************************* Handler Method for Status ************************************************

func (s *apiServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ************************************ Handler Methods for Wallets ************************************************

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

// **************************** Handler Methods for Categories ************************************************

func (s *apiServer) handleWallets(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.store.View(func(d storage.Data) {
			writeJSON(w, http.StatusOK, d.Wallets)
		})
	case http.MethodPost:
		var req struct {
			Mode                models.CreationMode `json:"mode"`
			Name                string              `json:"name"`
			MonthlyAllocation   *float64            `json:"monthly_allocation,omitempty"`
			TargetAmount        float64             `json:"target_amount,omitempty"`
			EOMSweepDestination string              `json:"eom_sweep_destination,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
			return
		}
		created, err := s.walletService.CreateWallet(req.Mode, req.Name, req.TargetAmount, req.EOMSweepDestination)
		if err != nil {
			writeAPIError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, created)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *apiServer) handleWalletBreakdowns(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	scope := r.URL.Query().Get("scope")
	now := time.Now()

	switch scope {
	case "", "all":
		monthly, err := s.walletService.AllMonthlyBreakdowns(now)
		if err != nil {
			writeAPIError(w, err)
			return
		}
		global, err := s.walletService.AllGlobalBreakdowns()
		if err != nil {
			writeAPIError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"monthly": monthly, "global": global})
	case "monthly":
		monthly, err := s.walletService.AllMonthlyBreakdowns(now)
		if err != nil {
			writeAPIError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, monthly)
	case "global":
		global, err := s.walletService.AllGlobalBreakdowns()
		if err != nil {
			writeAPIError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, global)
	default:
		http.Error(w, "invalid scope value", http.StatusBadRequest)
	}
}

func (s *apiServer) handleWalletByID(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, "/api/wallets/") {
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
		saved, err := s.walletService.UpdateWallet(id, req.Name, req.TargetAmount, req.EOMSweepDestination)
		if err != nil {
			writeAPIError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, saved)
	case http.MethodDelete:
		saved, err := s.walletService.DeleteWallet(id)
		if err != nil {
			writeAPIError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, saved)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// **************************** Handler Methods for Categories ************************************************

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

	case http.MethodGet:
		category, err := s.categoryService.GetCategoryByID(id)
		if err != nil {
			writeAPIError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, category)

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

// **************************Handler for triggering the end-of-month sweep manually***************************

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

// **************************Handler Methods for transferring funds between wallets***************************

func (s *apiServer) handleFundingOptions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	walletID := r.URL.Query().Get("wallet_id")
	amountParam := r.URL.Query().Get("amount")
	if walletID == "" || amountParam == "" {
		http.Error(w, "wallet_id and amount are required", http.StatusBadRequest)
		return
	}
	amount, err := strconv.ParseFloat(amountParam, 64)
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid amount: %v", err), http.StatusBadRequest)
		return
	}
	options, err := s.txService.EligibleFundingOptions(walletID, amount, time.Now())
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
		SourceWalletID    string                   `json:"source_wallet_id"`
		TargetWalletID    string                   `json:"target_wallet_id"`
		Amount            float64                  `json:"amount"`
		Category          string                   `json:"category,omitempty"`
		Counterparty      string                   `json:"counterparty,omitempty"`
		PaymentMode       models.PaymentMode       `json:"payment_mode,omitempty"`
		PaymentInstrument models.PaymentInstrument `json:"payment_instrument,omitempty"`
		Date              string                   `json:"date,omitempty"`
		Details           string                   `json:"details,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
		return
	}
	if req.Mechanism != models.SelfTransfer && req.Mechanism != models.InterWalletLoan {
		http.Error(w, "mechanism must be Self Transfer or Inter-Wallet Loan", http.StatusBadRequest)
		return
	}
	date, err := models.ParseFlexibleDate(req.Date)
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid date: %v", err), http.StatusBadRequest)
		return
	}
	funding := domain.FundingOption{
		Mechanism:      req.Mechanism,
		SourceWalletID: req.SourceWalletID,
	}
	debit := models.Transaction{
		Type:              models.Debit,
		Amount:            req.Amount,
		SourceWalletID:    req.TargetWalletID,
		Category:          req.Category,
		Counterparty:      req.Counterparty,
		PaymentMode:       req.PaymentMode,
		PaymentInstrument: req.PaymentInstrument,
		Date:              date,
		Details:           req.Details,
	}
	fundingTx, debitTx, err := s.txService.FundAndCreateDebit(funding, debit)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]models.Transaction{"funding": fundingTx, "debit": debitTx})
}

// *******************************Handler Methods for Loans************************************************

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

func (s *apiServer) handleInterWalletLoans(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	entries, err := s.loanService.AllInterWalletLoans()
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, entries)
}

func (s *apiServer) handleSettleInterWalletLoan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		BorrowerWalletID string  `json:"borrower_wallet_id"`
		LenderWalletID   string  `json:"lender_wallet_id"`
		Amount           float64 `json:"amount"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
		return
	}
	saved, err := s.loanService.SettleInterWalletLoan(req.BorrowerWalletID, req.LenderWalletID, req.Amount, time.Now())
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, saved)
}

// *******************Handler Methods for To-Do List (Tasks) Feature***************************

func (s *apiServer) handleTasks(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet, http.MethodPost, http.MethodDelete:
		s.proxyTasks(w, r, "")
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *apiServer) handleTaskbyID(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, "/api/tasks/") {
		http.NotFound(w, r)
		return
	}
	id := path.Base(r.URL.Path)
	switch r.Method {
	case http.MethodGet, http.MethodPut, http.MethodDelete:
		s.proxyTasks(w, r, "/"+id)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *apiServer) proxyTasks(w http.ResponseWriter, r *http.Request, pathSuffix string) {
	resp, err := s.tasks.Forward(r.Context(), r.Method, pathSuffix, r.Body, r.Header)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{
			"error": fmt.Sprintf("task server unavailable: %v", err),
		})
		return
	}
	w.Header().Set("Content-Type", resp.ContentType)
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(resp.Body)
}

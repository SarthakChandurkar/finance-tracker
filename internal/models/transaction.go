package models

import (
	"errors"
	"time"
)

// TransactionType is one of the 6 kinds of transaction in the app (A1.1).
// We define it as a named string type (not a plain string) so the Go
// compiler helps us catch typos — you can only assign one of the
// constants below to a TransactionType variable, not any random string.
type TransactionType string

const (
	Debit          TransactionType = "Debit"
	Credit         TransactionType = "Credit"
	Salary         TransactionType = "Salary"
	LoanReceived   TransactionType = "Loan Received"
	SelfTransfer   TransactionType = "Self Transfer"
	InterQuotaLoan TransactionType = "Inter-Quota Loan"
)

// BalanceEffect is auto-derived from Type (A1.1) — never set directly.
type BalanceEffect string

const (
	BalanceChanging    BalanceEffect = "Balance-Changing"
	NonBalanceChanging BalanceEffect = "Non-Balance-Changing"
)

// BalanceEffect is a METHOD on TransactionType. In Go, "func (t TransactionType)"
// before the function name means: this function is attached to TransactionType,
// and can be called as someType.BalanceEffect() instead of BalanceEffect(someType).
func (t TransactionType) BalanceEffect() BalanceEffect {
	switch t {
	case SelfTransfer, InterQuotaLoan:
		return NonBalanceChanging
	default:
		return BalanceChanging
	}
}

// PaymentMode — A3.1
type PaymentMode string

const (
	PaymentModeSelf       PaymentMode = "Self"
	PaymentModeOnBehalfOf PaymentMode = "On Behalf of Other"
)

// PaymentInstrument — A1.2
type PaymentInstrument string

const (
	InstrumentCash         PaymentInstrument = "Cash"
	InstrumentUPI          PaymentInstrument = "UPI"
	InstrumentCard         PaymentInstrument = "Card"
	InstrumentBankTransfer PaymentInstrument = "Bank Transfer"
	InstrumentCheque       PaymentInstrument = "Cheque"
)

const DefaultCategory = "Miscellaneous"

// Transaction is THE single schema (see the "Single Schema Principle" note
// at the top of Part A1). Quotas, the counterparty ledger, and category
// totals are all just different filters/aggregations over a list of these.
//
// The `json:"..."` text after each field is a "struct tag". It tells Go's
// encoding/json package what key name to use when this struct is written
// to or read from a JSON file. `omitempty` means: if the field is empty
// (zero value), leave it out of the JSON entirely instead of writing "".
type Transaction struct {
	ID                 string            `json:"id"`
	Type               TransactionType   `json:"type"`
	Amount             float64           `json:"amount"`
	SourceQuotaID      string            `json:"source_quota_id,omitempty"`
	DestinationQuotaID string            `json:"destination_quota_id,omitempty"`
	Category           string            `json:"category,omitempty"`
	Counterparty       string            `json:"counterparty,omitempty"`
	PaymentMode        PaymentMode       `json:"payment_mode,omitempty"`
	PaymentInstrument  PaymentInstrument `json:"payment_instrument,omitempty"`
	Date               time.Time         `json:"date"`
	Details            string            `json:"details,omitempty"`
}

// BalanceEffect on a Transaction just delegates to its Type. This is the
// "automatic" field from the A1.2 table — we never store it, we compute it
// on demand so it can never drift out of sync with Type.
func (tx Transaction) BalanceEffect() BalanceEffect {
	return tx.Type.BalanceEffect()
}

// ApplyDefaults fills in the default values listed in the A1.2 table.
// Called once, right before a new transaction is saved.
func (tx *Transaction) ApplyDefaults() {
	if tx.Type == "" {
		tx.Type = Debit
	}
	if tx.PaymentMode == "" {
		tx.PaymentMode = PaymentModeSelf
	}
	if tx.Date.IsZero() {
		tx.Date = time.Now()
	}
	if tx.Type == Debit && tx.Category == "" {
		tx.Category = DefaultCategory
	}
}

// Validate enforces the "Required?" column of the A1.2 table.
// It returns a Go `error`. Returning nil means "no error, all good" —
// this is the standard Go pattern instead of throwing exceptions.
func (tx Transaction) Validate() error {
	if tx.Amount <= 0 {
		return errors.New("amount is compulsory and must be greater than zero")
	}

	switch tx.Type {
	case Debit, SelfTransfer, InterQuotaLoan:
		if tx.SourceQuotaID == "" {
			return errors.New("source quota is required for this transaction type")
		}
	}

	switch tx.Type {
	case SelfTransfer, InterQuotaLoan:
		if tx.DestinationQuotaID == "" {
			return errors.New("destination quota is required for Self Transfer / Inter-Quota Loan")
		}
	}

	if tx.Type == Debit && tx.Category == "" {
		return errors.New("category is compulsory for Debit transactions")
	}

	return nil
}
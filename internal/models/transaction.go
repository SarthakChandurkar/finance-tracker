package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// dateOnlyLayout is what an HTML <input type="date"> sends: just
// "2026-07-19", with no time-of-day or timezone component at all.
const dateOnlyLayout = "2006-01-02"

// ParseFlexibleDate accepts either a full RFC3339 timestamp (what our own
// JSON export / GET responses produce, e.g. "2026-07-19T12:00:00+05:30")
// or a plain "YYYY-MM-DD" date (what browsers send from a native date
// picker). Go's encoding/json only understands the former by default,
// which is why date-only input used to fail with a parse error.
func ParseFlexibleDate(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t, nil
	}
	if t, err := time.Parse(dateOnlyLayout, s); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("could not parse date %q (expected YYYY-MM-DD or RFC3339)", s)
}

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

// type Money float64

// func (m Money) MarshalJSON() ([]byte, error) {
// 	// %.2f forces exactly two decimal places
// 	formatted := fmt.Sprintf("%.2f", m)
// 	return []byte(formatted), nil
// }

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

// UnmarshalJSON overrides the default decoding so the Date field accepts
// a plain "YYYY-MM-DD" (from an HTML date input) in addition to the full
// RFC3339 timestamps Go's time.Time normally requires. Everything else
// decodes exactly as it would by default — we only intercept "date".
func (tx *Transaction) UnmarshalJSON(data []byte) error {
	type Alias Transaction // same fields, but without this UnmarshalJSON method, to avoid infinite recursion
	aux := struct {
		Date string `json:"date"`
		*Alias
	}{
		Alias: (*Alias)(tx),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	parsed, err := ParseFlexibleDate(aux.Date)
	if err != nil {
		return err
	}
	tx.Date = parsed
	return nil
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

	switch tx.Type {
	case Credit, Salary:
		if tx.DestinationQuotaID == "" {
			return errors.New("destination quota is required for Credit and Salary transactions — otherwise the money isn't credited to any quota")
		}
	}
	// Loan Received's Destination Quota is intentionally OPTIONAL: unlike
	// Credit/Salary, a Loan Received is already tracked without one via its
	// (required) Counterparty in the Loan Ledger. If a destination is also
	// given, it's credited there too (see monthlyAvailable/globalAccumulated);
	// if not, the transaction still exists purely as a ledger entry.

	if tx.Type == LoanReceived && tx.Counterparty == "" {
		return errors.New("counterparty is required for Loan Received transactions, so it can be tracked in the loan ledger")
	}

	if tx.Type == Debit && tx.Category == "" {
		return errors.New("category is compulsory for Debit transactions")
	}

	return nil
}

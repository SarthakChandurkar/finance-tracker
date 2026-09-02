package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

const dateOnlyLayout = "2006-01-02"

// ParseFlexibleDate parses a plain calendar date input (e.g. from a date
// picker), which never carries a time component — the actual transaction
// time is captured separately in Transaction.RecordedAt (see ApplyDefaults).
// It is intentionally strict to YYYY-MM-DD: there is no "time" to lose here
// because none is expected on this field.
func ParseFlexibleDate(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse(dateOnlyLayout, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("could not parse date %q (expected YYYY-MM-DD)", s)
	}
	return t, nil
}

type TransactionType string

const (
	Debit           TransactionType = "Debit"
	Credit          TransactionType = "Credit"
	Salary          TransactionType = "Salary"
	LoanReceived    TransactionType = "Loan Received"
	SelfTransfer    TransactionType = "Self Transfer"
	InterWalletLoan TransactionType = "Inter-Wallet Loan"
)

type BalanceEffect string

const (
	BalanceChanging    BalanceEffect = "Balance-Changing"
	NonBalanceChanging BalanceEffect = "Non-Balance-Changing"
)

func (t TransactionType) BalanceEffect() BalanceEffect {
	switch t {
	case SelfTransfer, InterWalletLoan:
		return NonBalanceChanging
	default:
		return BalanceChanging
	}
}

type PaymentMode string

const (
	PaymentModeSelf       PaymentMode = "Self"
	PaymentModeOnBehalfOf PaymentMode = "On Behalf of Other"
)

type PaymentInstrument string

const (
	InstrumentUPI          PaymentInstrument = "UPI"
	InstrumentCash         PaymentInstrument = "Cash"
	InstrumentCard         PaymentInstrument = "Card"
	InstrumentBankTransfer PaymentInstrument = "Bank Transfer"
	InstrumentCheque       PaymentInstrument = "Cheque"
)

const DefaultCategory = "Settled"

type Transaction struct {
	ID                  string            `json:"id"`
	UserID              string            `json:"user_id"`
	Type                TransactionType   `json:"type"`
	Amount              float64           `json:"amount"`
	SourceWalletID      string            `json:"source_wallet_id,omitempty"`
	DestinationWalletID string            `json:"destination_wallet_id,omitempty"`
	Category            string            `json:"category,omitempty"`
	Counterparty        string            `json:"counterparty,omitempty"`
	PaymentMode         PaymentMode       `json:"payment_mode,omitempty"`
	PaymentInstrument   PaymentInstrument `json:"payment_instrument,omitempty"`
	Date                time.Time         `json:"date"`
	Details             string            `json:"details,omitempty"`

	// RecordedAt is the exact instant the transaction was recorded, always
	// stored in UTC/GMT (so the backend has a single, unambiguous instant
	// regardless of which timezone the server or client is in). It is
	// additive and optional (json "omitempty") so records written before
	// this field existed — which have no "recorded_at" key — continue to
	// decode fine with a zero value.
	//
	// Converting this to a 12-hour clock string in the viewer's local
	// timezone is a display concern and is left to the frontend
	// (RecordedAt.In(userLocation).Format("03:04 PM") client-side).
	RecordedAt time.Time `json:"recorded_at,omitempty"`
}

func (tx *Transaction) UnmarshalJSON(data []byte) error {
	type Alias Transaction
	aux := struct {
		Date   string      `json:"date"`
		Amount json.Number `json:"amount"`
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

	if raw := aux.Amount.String(); raw != "" {
		amount, err := ParseMoneyToken(raw)
		if err != nil {
			return err
		}
		tx.Amount = amount
	}

	return nil
}

func (tx Transaction) BalanceEffect() BalanceEffect {
	return tx.Type.BalanceEffect()
}

func (tx Transaction) GetID() string {
	return tx.ID
}

// OwnerID identifies which user this transaction belongs to.
func (tx Transaction) OwnerID() string {
	return tx.UserID
}

func (tx *Transaction) ApplyDefaults() {
	if tx.Type == "" {
		tx.Type = Debit
	}
	if tx.PaymentMode == "" {
		tx.PaymentMode = PaymentModeSelf
	}
	if tx.Date.IsZero() {
		tx.Date = time.Now().UTC()
	}
	if tx.RecordedAt.IsZero() {
		// The actual moment the transaction is recorded, captured in
		// UTC/GMT. Marshals as RFC3339 with a "Z" suffix, e.g.
		// "2026-09-02T08:35:32Z" — an unambiguous instant that any
		// frontend can convert into the viewer's local 12-hour time.
		tx.RecordedAt = time.Now().UTC()
	}
	if tx.Type == Debit && tx.Category == "" {
		tx.Category = DefaultCategory
	}
}

func (tx Transaction) Validate() error {
	if tx.Amount <= 0 {
		return errors.New("amount is compulsory and must be greater than zero")
	}
	if err := ValidateMoneyFloat(tx.Amount); err != nil {
		return err
	}

	switch tx.Type {
	case Debit, SelfTransfer, InterWalletLoan:
		if tx.SourceWalletID == "" {
			return errors.New("source wallet is required for this transaction type")
		}
	}

	switch tx.Type {
	case SelfTransfer, InterWalletLoan:
		if tx.DestinationWalletID == "" {
			return errors.New("destination wallet is required for Self Transfer / Inter-Wallet Loan")
		}
	}

	switch tx.Type {
	case Credit, Salary:
		if tx.DestinationWalletID == "" {
			return errors.New("destination wallet is required for Credit and Salary transactions — otherwise the money isn't credited to any wallet")
		}
	}

	if tx.Type == LoanReceived && tx.Counterparty == "" {
		return errors.New("counterparty is required for Loan Received transactions, so it can be tracked in the loan ledger")
	}

	return nil
}

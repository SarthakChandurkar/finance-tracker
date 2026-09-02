package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

const dateOnlyLayout = "2006-01-02"

// ParseFlexibleDate accepts either:
//   - a plain calendar date "YYYY-MM-DD" (e.g. from a date picker), or
//   - a full RFC3339 / RFC3339Nano timestamp.
//
// In the plain-date case (or an RFC3339 timestamp whose clock component is
// exactly midnight, meaning no real time was supplied), it enriches the
// parsed date with the *current* clock time — so the transaction still
// records the exact moment it was recorded, not a bare midnight timestamp.
// If a real time-of-day is present in the input (RFC3339 with a non-zero
// clock), that exact time is preserved as-is.
func ParseFlexibleDate(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}

	enrichWithExactTime := func(t time.Time) time.Time {
		// Evaluate "now" in the same location as the parsed date (usually
		// UTC for date-only input) so the enriched timestamp stays
		// self-consistent.
		now := time.Now().In(t.Location())
		return time.Date(t.Year(), t.Month(), t.Day(), now.Hour(), now.Minute(), now.Second(), now.Nanosecond(), t.Location())
	}

	if t, err := time.Parse(dateOnlyLayout, s); err == nil {
		return enrichWithExactTime(t), nil
	}

	if t, err := time.Parse(time.RFC3339, s); err == nil {
		if t.Hour() == 0 && t.Minute() == 0 && t.Second() == 0 {
			return enrichWithExactTime(t), nil
		}
		return t, nil
	}

	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		if t.Hour() == 0 && t.Minute() == 0 && t.Second() == 0 {
			return enrichWithExactTime(t), nil
		}
		return t, nil
	}

	return time.Time{}, fmt.Errorf("could not parse date %q (expected YYYY-MM-DD or RFC3339)", s)
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
		tx.Date = time.Now()
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

package models

import (
	"fmt"
	"strconv"
	"strings"
)

// Money-related helpers.
//
// Transaction.Amount stays a float64 (unchanged schema), but every entry
// point that sets it — JSON input or arithmetic performed in Go code — is
// routed through these helpers so an amount can never end up carrying more
// than 2 decimal places.

const moneyDecimalPlaces = 2

// ParseMoneyToken validates a monetary amount given as the *exact* text it
// appeared as (e.g. the raw JSON number token) and returns it as a float64.
// It rejects amounts with more than 2 decimal places — e.g. "45.6789" or
// "45.678" — while accepting "45.67", "45", "67.08", "45.9800", "45.980".
func ParseMoneyToken(raw string) (float64, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0, fmt.Errorf("amount is required")
	}

	body := s
	if strings.HasPrefix(body, "+") || strings.HasPrefix(body, "-") {
		body = body[1:]
	}
	if strings.ContainsAny(body, "eE") {
		return 0, fmt.Errorf("amount %q must be a plain decimal number, not scientific notation", raw)
	}

	intPart, fracPart, hasFrac := strings.Cut(body, ".")
	if intPart == "" || !isDigits(intPart) {
		return 0, fmt.Errorf("amount %q is not a valid number", raw)
	}
	if hasFrac {
		if !isDigits(fracPart) {
			return 0, fmt.Errorf("amount %q is not a valid number", raw)
		}
		if len(fracPart) > moneyDecimalPlaces {
			return 0, fmt.Errorf("amount %q has more than %d decimal places; amounts can have at most %d decimal places", raw, moneyDecimalPlaces, moneyDecimalPlaces)
		}
	}

	value, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("amount %q is not a valid number: %w", raw, err)
	}
	return value, nil
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// ValidateMoneyFloat checks that a float64 amount does not carry more than
// 2 decimal places of precision. It formats the amount using its shortest
// exact round-trip decimal representation, so trailing zeros such as
// 45.9800 are correctly treated as 45.98, while genuine extra precision
// such as 45.6789 is rejected. Use this as a defense-in-depth check
// wherever an Amount could be set outside of JSON unmarshalling (e.g. set
// directly in Go code).
func ValidateMoneyFloat(amount float64) error {
	str := strconv.FormatFloat(amount, 'f', -1, 64)
	_, fracPart, hasFrac := strings.Cut(strings.TrimPrefix(str, "-"), ".")
	if hasFrac && len(fracPart) > moneyDecimalPlaces {
		return fmt.Errorf("amount %v has more than %d decimal places; amounts can have at most %d decimal places", amount, moneyDecimalPlaces, moneyDecimalPlaces)
	}
	return nil
}

// RoundMoney rounds amount to 2 decimal places using half-away-from-zero
// rounding. Apply this to the result of any arithmetic performed on money
// values (addition, subtraction, splits, etc.) so binary floating point
// noise (e.g. 0.1 + 0.2 == 0.30000000000000004) never leaks into a stored
// amount.
func RoundMoney(amount float64) float64 {
	const scale = 100.0
	if amount >= 0 {
		return float64(int64(amount*scale+0.5)) / scale
	}
	return float64(int64(amount*scale-0.5)) / scale
}

// AddMoney adds two money amounts, returning a result rounded to 2 decimal
// places.
func AddMoney(a, b float64) float64 {
	return RoundMoney(a + b)
}

// SubMoney subtracts b from a, returning a result rounded to 2 decimal
// places.
func SubMoney(a, b float64) float64 {
	return RoundMoney(a - b)
}

// Money is a small integer-backed (smallest-unit, e.g. paise) type for code
// that needs to chain several operations on a monetary value without
// worrying about float drift at every step. Internally it stores the
// amount as an integer count of the smallest currency unit (1 Rupee = 100
// paise), so Add/Sub are always exact — no rounding needed after each op.
type Money struct {
	units int64 // amount * 100, rounded to the nearest integer
}

// NewMoney converts a float64 amount into a Money value, validating that it
// does not carry more than 2 decimal places.
func NewMoney(amount float64) (Money, error) {
	if err := ValidateMoneyFloat(amount); err != nil {
		return Money{}, err
	}
	if amount >= 0 {
		return Money{units: int64(amount*100 + 0.5)}, nil
	}
	return Money{units: int64(amount*100 - 0.5)}, nil
}

// Float64 converts the Money value back to a float64 (e.g. to assign to
// Transaction.Amount).
func (m Money) Float64() float64 {
	return float64(m.units) / 100
}

// Add returns m + other. Exact — no rounding required.
func (m Money) Add(other Money) Money {
	return Money{units: m.units + other.units}
}

// Sub returns m - other. Exact — no rounding required.
func (m Money) Sub(other Money) Money {
	return Money{units: m.units - other.units}
}

// String renders the amount with exactly 2 decimal places, e.g. "45.90".
func (m Money) String() string {
	sign := ""
	units := m.units
	if units < 0 {
		sign = "-"
		units = -units
	}
	return fmt.Sprintf("%s%d.%02d", sign, units/100, units%100)
}

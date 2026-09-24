package currency

import (
	"strings"

	"github.com/hanzoai/money"
)

// Money is an amount and the asset it is denominated in, as ONE value.
//
// They never travel apart. A bare number beside a currency code is the shape
// that truncates wei into a cents column, because the pairing is left to the
// caller to remember.
//
// Amount is an exact decimal string in the asset's MAJOR units ("0.0731",
// "250.00"). A string, because an 18-decimal asset does not fit an int64 — one
// ETH is 1e18 wei and int64 stops at nine of them. Precision is not this type's
// to know: Exact reads the scale from Type.Decimals, the one table that owns it.
type Money struct {
	Amount string `json:"amount"`
	Asset  Type   `json:"asset"`
}

// Exact lifts this into hanzoai/money for arithmetic — arbitrary precision, at
// the asset's own scale, with no int64 on the path.
func (m Money) Exact() (money.Amount, error) { return m.Asset.ParseAmount(m.Amount) }

// IsZero reports whether no amount was given at all.
func (m Money) IsZero() bool { return strings.TrimSpace(m.Amount) == "" }

// Exact renders an exact amount back into a Money.
func Exact(a money.Amount) Money {
	return Money{Amount: a.MajorString(), Asset: Type(strings.ToLower(a.Currency().Code))}
}

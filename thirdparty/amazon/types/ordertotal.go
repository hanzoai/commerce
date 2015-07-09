package types

type OrderTotal struct {
	CurrencyCode string // Three-digit ISO 4217. Required.
	Amount       string // Decimal places must be appropriate for CurrencyCode. Period is the only valid separator.
}

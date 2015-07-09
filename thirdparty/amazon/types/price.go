package types

type Price struct {
	Amount       string // Currency amount. Decimal places are set by CurrencyCode. Period is the only valid decimal separator.
	CurrencyCode string // Three-digit ISO 4217
}

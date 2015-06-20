package amazon

type OrderTotal struct {
	CurrencyCode string //required - three-digit ISO 4217
	Amount       string //Decimal places must be appropriate for CurrencyCode. Period is the only valid separator.
}

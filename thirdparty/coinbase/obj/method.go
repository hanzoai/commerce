package obj

type Method struct {
	Id            string // Resource Id
	Type          string // Payment method type. Enum. Possibilities: ach_bank_account, sepa_bank_account, ideal_bank_account, fiat_account, bank_wire, credit_card, secure3d_card, eft_bank_account, interac
	Name          string // Payment method type.
	Currency      string // Primary currency
	Primary_Buy   bool   // Is primary buying method?
	Primary_Sell  bool   // Is primary selling method?
	Allow_Buy     bool   // Is buying allowed with the method?
	Allow_Sell    bool   // Is selling allowed with this method?
	Instant_buy   bool   // Is this method allowed for instant buys?
	Instant_sell  bool   // Is this method allowed for instant sells?
	Created_At    string // timestamp for creation time
	Sold_At       string // Timestamp for instant selling
	Resource      string // Constant: Payment_Method
	Resource_Path string
}

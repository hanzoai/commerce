package obj

type Transaction struct {
	Id   string // Resource ID
	Type string // Transaction Type.
	/* Enum, possible values:
	send, request, transfer, buy, sell,
	fiat_deposit, fiat_withdrawal,
	exchange_deposit, exchange_withdrawal, vault_withdrawal.
	Source: https://developers.coinbase.com/api/v2#transactions */
	Status string // Status
	/* Enum, possible values:
	pending, completed, failed, expired, canceled
	waiting_for_signature, waiting_for_clearing
	Source: https://developers.coinbase.com/api/v2#transactions */
	Amount           string // Amount in bitcoin, litecoin, or ethereum
	Native_Amount    string // Amount in user's native currency
	Description      string // User defined description
	Instant_Exchange bool   // Indicator if the transaction was instant exchanged (received into a bitcoin address for a fiat account)
	Created_At       string // Timestamp for creation
	Updated_At       string // Timestamp for update
	Resource         string
	Resource_Path    string
}

package obj

type Account struct {
	Id             string // Resource Id
	Name           string // User or system defined name
	Primary        bool   // Primary account
	Type           string // Account type. Available values: wallet, fiat, multisig, vault, multisig_vault
	Currency       string // Account's currency
	Balance        string // Balance in BTC and ETH
	Native_Balance string // Balance in user's native currency
	Created_At     string // Timestamp of creation
	Updated_At     string // Timestamp of update
	Resource       string
	Resource_Path  string
}

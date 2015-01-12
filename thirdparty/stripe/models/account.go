package models

type Account struct {
	BusinessName        string   `json:"business_name"`
	ChargesEnabled      bool     `json:"charges_enabled"`
	Country             string   `json:"country"`
	CurrenciesSupported []string `json:"currencies_supported"`
	DefaultCurrency     string   `json:"default_currency"`
	DetailsSubmitted    bool     `json:"details_submitted"`
	DisplayName         string   `json:"display_name"`
	Email               string   `json:"email"`
	ID                  string   `json:"id"`
	Object              string   `json:"object"`
	StatementDescriptor string   `json:"statement_descriptor"`
	Timezone            string   `json:"timezone"`
	TransfersEnabled    bool     `json:"transfers_enabled"`
}

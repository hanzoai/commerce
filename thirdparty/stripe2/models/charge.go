package models

type Charge struct {
	Amount             float64 `json:"amount"`
	AmountRefunded     float64 `json:"amount_refunded"`
	BalanceTransaction string  `json:"balance_transaction"`
	Captured           bool    `json:"captured"`
	Card               struct {
		AddressCity       string  `json:"address_city"`
		AddressCountry    string  `json:"address_country"`
		AddressLine1      string  `json:"address_line1"`
		AddressLine1Check string  `json:"address_line1_check"`
		AddressLine2      string  `json:"address_line2"`
		AddressState      string  `json:"address_state"`
		AddressZip        string  `json:"address_zip"`
		AddressZipCheck   string  `json:"address_zip_check"`
		Brand             string  `json:"brand"`
		Country           string  `json:"country"`
		Customer          string  `json:"customer"`
		CvcCheck          string  `json:"cvc_check"`
		DynamicLast4      string  `json:"dynamic_last4"`
		ExpMonth          float64 `json:"exp_month"`
		ExpYear           float64 `json:"exp_year"`
		Fingerprint       string  `json:"fingerprint"`
		Funding           string  `json:"funding"`
		ID                string  `json:"id"`
		Last4             string  `json:"last4"`
		Name              string  `json:"name"`
		Object            string  `json:"object"`
	} `json:"card"`
	Created        float64           `json:"created"`
	Currency       string            `json:"currency"`
	Customer       string            `json:"customer"`
	Description    string            `json:"description"`
	Dispute        string            `json:"dispute"`
	FailureCode    string            `json:"failure_code"`
	FailureMessage string            `json:"failure_message"`
	Fee            float64           `json:"fee"`
	FraudDetails   map[string]string `json:"fraud_details"`
	ID             string            `json:"id"`
	Invoice        string            `json:"invoice"`
	Livemode       bool              `json:"livemode"`
	Metadata       map[string]string `json:"metadata"`
	Object         string            `json:"object"`
	Paid           bool              `json:"paid"`
	ReceiptEmail   string            `json:"receipt_email"`
	ReceiptNumber  string            `json:"receipt_number"`
	Refunded       bool              `json:"refunded"`
	Refunds        struct {
		Data []struct {
			Amount             float64           `json:"amount"`
			BalanceTransaction string            `json:"balance_transaction"`
			Charge             string            `json:"charge"`
			Created            float64           `json:"created"`
			Currency           string            `json:"currency"`
			ID                 string            `json:"id"`
			Metadata           map[string]string `json:"metadata"`
			Object             string            `json:"object"`
			Reason             string            `json:"reason"`
			ReceiptNumber      string            `json:"receipt_number"`
		} `json:"data"`
		HasMore    bool    `json:"has_more"`
		Object     string  `json:"object"`
		TotalCount float64 `json:"total_count"`
		URL        string  `json:"url"`
	} `json:"refunds"`
	Shipping             string `json:"shipping"`
	StatementDescription string `json:"statement_description"`
	StatementDescriptor  string `json:"statement_descriptor"`
}

package models

type Dispute struct {
	Amount              float64 `json:"amount"`
	BalanceTransactions []struct {
		Amount      float64 `json:"amount"`
		AvailableOn float64 `json:"available_on"`
		Created     float64 `json:"created"`
		Currency    string  `json:"currency"`
		Description string  `json:"description"`
		Fee         float64 `json:"fee"`
		FeeDetails  []struct {
			Amount      float64     `json:"amount"`
			Application interface{} `json:"application"`
			Currency    string      `json:"currency"`
			Description string      `json:"description"`
			Type        string      `json:"type"`
		} `json:"fee_details"`
		ID     string  `json:"id"`
		Net    float64 `json:"net"`
		Object string  `json:"object"`
		Source string  `json:"source"`
		Status string  `json:"status"`
		Type   string  `json:"type"`
	} `json:"balance_transactions"`
	Charge   string  `json:"charge"`
	Created  float64 `json:"created"`
	Currency string  `json:"currency"`
	Evidence struct {
		AccessActivityLog            interface{} `json:"access_activity_log"`
		BillingAddress               interface{} `json:"billing_address"`
		CancellationPolicy           interface{} `json:"cancellation_policy"`
		CancellationPolicyDisclosure interface{} `json:"cancellation_policy_disclosure"`
		CancellationRebuttal         interface{} `json:"cancellation_rebuttal"`
		CustomerCommunication        interface{} `json:"customer_communication"`
		CustomerEmailAddress         interface{} `json:"customer_email_address"`
		CustomerName                 interface{} `json:"customer_name"`
		CustomerPurchaseIp           interface{} `json:"customer_purchase_ip"`
		CustomerSignature            interface{} `json:"customer_signature"`
		DuplicateChargeDocumentation interface{} `json:"duplicate_charge_documentation"`
		DuplicateChargeExplanation   interface{} `json:"duplicate_charge_explanation"`
		DuplicateChargeID            interface{} `json:"duplicate_charge_id"`
		ProductDescription           interface{} `json:"product_description"`
		Receipt                      interface{} `json:"receipt"`
		RefundPolicy                 interface{} `json:"refund_policy"`
		RefundPolicyDisclosure       interface{} `json:"refund_policy_disclosure"`
		RefundRefusalExplanation     interface{} `json:"refund_refusal_explanation"`
		ServiceDate                  interface{} `json:"service_date"`
		ServiceDocumentation         interface{} `json:"service_documentation"`
		ShippingAddress              interface{} `json:"shipping_address"`
		ShippingDate                 interface{} `json:"shipping_date"`
		ShippingDocumentation        interface{} `json:"shipping_documentation"`
		ShippingTrackingNumber       interface{} `json:"shipping_tracking_number"`
		UncategorizedFile            interface{} `json:"uncategorized_file"`
		UncategorizedText            interface{} `json:"uncategorized_text"`
	} `json:"evidence"`
	EvidenceDetails struct {
		DueBy           float64 `json:"due_by"`
		SubmissionCount float64 `json:"submission_count"`
	} `json:"evidence_details"`
	IsChargeRefundable bool     `json:"is_charge_refundable"`
	Livemode           bool     `json:"livemode"`
	Metadata           struct{} `json:"metadata"`
	Object             string   `json:"object"`
	Reason             string   `json:"reason"`
	Status             string   `json:"status"`
}

var DisputeStatuses = struct {
	WarningNeedsResponse string
	WarningUnderReview   string
	NeedsResponse        string
	ResponseDisabled     string
	UnderReview          string
	ChargeRefunded       string
	Won                  string
	Lost                 string
}{
	"warning_needs_response",
	"warning_under_review",
	"needs_response",
	"response_disable",
	"under_review",
	"charge_refunded",
	"won",
	"lost",
}

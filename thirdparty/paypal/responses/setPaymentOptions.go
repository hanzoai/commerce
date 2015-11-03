package responses

// reference url: https://developer.paypal.com/docs/classic/api/adaptive-payments/SetPaymentOptions_API_Operation/
type SetPaymentOptionsResponse struct {
	Ack           string // Acknowledgement code
	Build         string // Build number, used by Paypal tech support
	CorrelationId string // Correlation identifier, used by paypal tech support
	Timestamp     string // Date stamp
	Error         string // error code
}

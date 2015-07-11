package types

type MerchantConfig struct {
	UserConfirmationUrl string `json:"user_confirmation_url"`     // Url that the customer is sent after confirming their payment with Affirm.
	UserCancelUrl       string `json:"user_cancel_url,omitempty"` // Url that the customer is sent to if the customer chooses to cancel the affirm payment before completion.
}

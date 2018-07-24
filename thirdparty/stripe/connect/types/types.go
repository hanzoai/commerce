package types

type Token struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	PublishableKey   string `json:"stripe_publishable_key"`
	UserId           string `json:"stripe_user_id"`
	TokenType        string `json:"token_type"`
	Scope            string `json:"scope"`
	Livemode         bool   `json:"livemode"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

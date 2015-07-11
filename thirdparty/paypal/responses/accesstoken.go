package responses

// Note: Unlike many other API reference docs, Paypal does not provide a listing of comments and such on
// many of its json objects.  The comments in this file are personal observations from the examples,
// rather than lifted from the reference docs like many similar files.

type AccessTokenResponse struct {
	Scope       string `json:"scope"`        // The paths the access token is valid for.
	AccessToken string `json:"access_token"` // The requested access token.
	TokenType   string `json:"token_type"`   // The type of token included.  The example shows 'Bearer', which presumably is a bit like a bearer bond.  Whomever has it is the rightful owner.
	AppId       string `json:"app_id"`       // Super poorly documented.  I presume it relates to the applications you're allowed to hit in Scope.
	ExpiresIn   int    `json:"expires_in"`   // The number of seconds before the given access token expires.
}

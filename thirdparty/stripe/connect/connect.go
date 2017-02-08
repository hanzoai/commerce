package connect

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"appengine"
	"appengine/urlfetch"

	"hanzo.io/config"
	"hanzo.io/util/json"
)

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

func GetToken(ctx appengine.Context, code string) (*Token, error) {
	client := urlfetch.Client(ctx)

	data := url.Values{}
	data.Set("client_secret", config.Stripe.SecretKey)
	data.Add("code", code)
	data.Add("grant_type", "authorization_code")

	tokenReq, err := http.NewRequest("POST", "https://connect.stripe.com/oauth/token", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}

	// try to post to OAuth API
	res, err := client.Do(tokenReq)
	defer res.Body.Close()
	if err != nil {
		return nil, err
	}

	// try and extract the json struct
	token := new(Token)
	if err := json.Decode(res.Body, token); err != nil {
		return nil, err
	}

	// Stripe returned an error
	if token.Error != "" {
		return nil, errors.New(token.Error)
	}

	return token, nil
}

func GetTestToken(ctx appengine.Context, refreshToken string) (*Token, error) {
	client := urlfetch.Client(ctx)

	data := url.Values{}
	data.Set("client_secret", config.Stripe.TestSecretKey)
	data.Add("refresh_token", refreshToken)
	data.Add("grant_type", "refresh_token")

	tokenReq, err := http.NewRequest("POST", "https://connect.stripe.com/oauth/token", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}

	// try to post to OAuth API
	res, err := client.Do(tokenReq)
	defer res.Body.Close()
	if err != nil {
		return nil, err
	}

	// try and extract the json struct
	token := new(Token)
	if err := json.Decode(res.Body, token); err != nil {
		return nil, err
	}

	// Stripe returned an error
	if token.Error != "" {
		return nil, errors.New(token.Error)
	}

	return token, nil
}

func GetTokens(ctx appengine.Context, code string) (*Token, *Token, error) {
	liveToken, err := GetToken(ctx, code)
	if err != nil {
		return nil, nil, err
	}

	// The development client id can only create test tokens, and you can only
	// have a single set of tokens at a time, thus return just the live token.
	if config.Stripe.ClientId == config.Stripe.DevelopmentClientId {
		return liveToken, liveToken, err
	}

	// In production, our users actually need both tokens created.
	testToken, err := GetTestToken(ctx, liveToken.RefreshToken)
	if err != nil {
		return nil, nil, err
	}

	return liveToken, testToken, nil
}

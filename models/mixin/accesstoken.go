package mixin

import (
	"errors"
	"math"
	"time"

	"crowdstart.io/util/bit"
	"crowdstart.io/util/token"
)

// Error for expired jti's
var ErrorExpiredToken = errors.New("This token is expired.")
var TokenNotFound = errors.New("Token not found.")

// AccessToken is a mixin for securing objects with an AccessToken
type AccessToken struct {
	// Model is a struct with a Model mixin
	Model model `json:"-" datastore:"-"`

	// JWT secret
	SecretKey []byte `json:"-"`

	// UseTokenId as JWT "jti" param, randomly generate upon generating a new key to expire all existing keys
	Tokens []token.Token `json:"-"`

	currentToken *token.Token
}

func (at *AccessToken) AddToken(name string, permissions bit.Mask) string {
	// Generate a new TokenId to invalidate previous key
	t := token.New(name, at.Model.Id(), permissions, at.SecretKey)
	t.IssuedAt = time.Now()
	at.Tokens = append(at.Tokens, *t)
	return t.String()
}

func (at *AccessToken) CompareToken(tok1, tok2 *token.Token) error {
	if tok1.Id != tok2.Id {
		return ErrorExpiredToken
	}

	if tok1.Permissions != tok2.Permissions {
		return ErrorExpiredToken
	}

	return nil
}

func (at *AccessToken) GetToken(accessToken string) (*token.Token, error) {
	tok, err := token.FromString(accessToken, at.SecretKey)
	if err != nil {
		return nil, err
	}

	// Try to fetch model using ModelId on token
	if err := at.Model.Get(tok.ModelId); err != nil {
		return nil, err
	}

	for _, _tok := range at.Tokens {
		if tok.Id == _tok.Id {
			return tok, at.CompareToken(tok, &_tok)
		}
	}
	return nil, TokenNotFound
}

func (at *AccessToken) RemoveToken(name string) {
	tokens := make([]token.Token, int(math.Max(float64(len(at.Tokens)-1), 0)))
	for _, tok := range at.Tokens {
		if tok.Name != name {
			tokens = append(tokens, tok)
		}
	}
	at.Tokens = tokens
}

func (at *AccessToken) ClearTokens() {
	at.Tokens = make([]token.Token, 0)
}

func (at *AccessToken) GetWithAccessToken(accessToken string) error {
	tok, err := at.GetToken(accessToken)
	if err != nil {
		return err
	}

	at.currentToken = tok

	return nil
}

func (at *AccessToken) HasPermission(mask bit.Mask) bool {
	return at.currentToken.HasPermission(mask)
}

package mixin

import (
	"errors"

	"crowdstart.io/util/bit"
	"crowdstart.io/util/token"
)

// Error for expired jti's
var ErrorExpiredToken = errors.New("This token is expired.")
var TokenNotFound = errors.New("Token not found.")

// AccessToken is a mixin for securing objects with an AccessToken
type AccessToken struct {
	// Entity is a struct with a Entity mixin
	Entity Entity `json:"-" datastore:"-"`

	// JWT secret
	SecretKey []byte `json:"-"`

	// UseTokenId as JWT "jti" param, randomly generate upon generating a new key to expire all existing keys
	Tokens []token.Token `json:"-"`

	currentToken *token.Token
}

func (at *AccessToken) AddToken(name string, permissions bit.Mask) string {
	// Generate a new TokenId to invalidate previous key
	t := token.New(name, at.Entity.Id(), permissions, at.SecretKey)
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

func (at *AccessToken) GetTokenByName(name string) (*token.Token, error) {
	for _, tok := range at.Tokens {
		if tok.Name == name {
			return &tok, nil
		}
	}
	return nil, TokenNotFound
}

func (at *AccessToken) MustGetTokenByName(name string) *token.Token {
	tok, err := at.GetTokenByName(name)
	if err != nil {
		panic(err)
	}
	return tok
}

func (at *AccessToken) GetToken(accessToken string) (*token.Token, error) {
	tok, err := token.FromString(accessToken, at.SecretKey)
	if err != nil {
		return nil, err
	}

	// Try to fetch model using EntityId on token
	if err := at.Entity.Get(tok.EntityId); err != nil {
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
	if len(at.Tokens) <= 0 {
		at.Tokens = make([]token.Token, 0)
		return
	}

	// Loop over tokens looking for token to delete, if we find it, remove from
	// tokens
	for i, tok := range at.Tokens {
		if tok.Name == name {
			// Delete from tokens
			at.Tokens = append(at.Tokens[:i], at.Tokens[i+1:]...)
			return
		}
	}
}

func (at *AccessToken) ClearTokens() {
	at.Tokens = make([]token.Token, 0)
}

func (at *AccessToken) GetWithAccessToken(accessToken string) (*token.Token, error) {
	tok, err := at.GetToken(accessToken)
	if err != nil {
		return tok, err
	}

	at.currentToken = tok

	return tok, nil
}

func (at *AccessToken) HasPermission(mask bit.Mask) bool {
	return at.currentToken.HasPermission(mask)
}

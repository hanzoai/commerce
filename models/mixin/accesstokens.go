package mixin

import (
	"errors"

	"hanzo.io/log"
	"hanzo.io/util/bit"
	token "hanzo.io/util/oldjwt"
)

// Error for expired jti's
var ErrorExpiredToken = errors.New("This token is expired")
var TokenNotFound = errors.New("Token not found")
var TokenNotFoundByName = errors.New("Token not found by name")

// AccessTokens is a mixin for securing objects with an AccessTokens
type AccessTokens struct {
	// Entity is a struct with a Entity mixin
	Entity Entity `json:"-" datastore:"-"`

	// JWT secret
	SecretKey []byte `json:"-"`

	// UseTokenId as JWT "jti" param, randomly generate upon generating a new key to expire all existing keys
	Tokens []token.Token `json:"-"`

	currentToken *token.Token
}

func (at *AccessTokens) Init(e Entity) {
	at.Entity = e
}

func (at *AccessTokens) AddToken(name string, permissions bit.Mask) string {
	// Generate a new TokenId to invalidate previous key
	t := token.New(name, at.Entity.Id(), permissions, at.SecretKey)
	at.Tokens = append(at.Tokens, *t)
	return t.String()
}

func (at *AccessTokens) CompareToken(tok1, tok2 *token.Token) error {
	if tok1.Id != tok2.Id {
		return ErrorExpiredToken
	}

	if tok1.Permissions != tok2.Permissions {
		return ErrorExpiredToken
	}

	return nil
}

func (at *AccessTokens) GetTokenByName(name string) (*token.Token, error) {
	for _, tok := range at.Tokens {
		if tok.Name == name {
			tok.Secret = at.SecretKey
			return &tok, nil
		}
	}

	log.Warn("Token not found by name '%s'", name)
	return nil, TokenNotFoundByName
}

func (at *AccessTokens) MustGetTokenByName(name string) *token.Token {
	tok, err := at.GetTokenByName(name)
	if err != nil {
		panic(err)
	}
	return tok
}

func (at *AccessTokens) GetToken(accessToken string) (*token.Token, error) {
	tok, err := token.FromString(accessToken, at.SecretKey)
	if err != nil {
		return tok, err
	}

	// Try to fetch model using EntityId on token
	if err := at.Entity.GetById(tok.EntityId); err != nil {
		return tok, err
	}

	for _, _tok := range at.Tokens {
		if tok.Id == _tok.Id {
			return tok, at.CompareToken(tok, &_tok)
		}
	}

	log.Warn("Token not found: %v", tok)
	return tok, TokenNotFound
}

func (at *AccessTokens) RemoveToken(name string) {
	num := len(at.Tokens)
	tokens := make([]token.Token, 0)
	if num <= 0 {
		at.Tokens = tokens
		return
	}

	// Loop over tokens looking for token to delete. We need to check every
	// token in case a duplicate was saved
	for i := 0; i < num; i++ {
		if at.Tokens[i].Name != name {
			tokens = append(tokens, at.Tokens[i])
		}
	}

	at.Tokens = tokens
}

func (at *AccessTokens) ClearTokens() {
	at.Tokens = make([]token.Token, 0)
}

func (at *AccessTokens) GetWithAccessToken(accessToken string) (*token.Token, error) {
	tok, err := at.GetToken(accessToken)
	if err != nil {
		log.Warn("Failed to get %v using token '%v': %v", at.Entity.Kind(), accessToken, err)
		return tok, err
	}

	at.currentToken = tok

	return tok, nil
}

func (at *AccessTokens) HasPermission(mask bit.Mask) bool {
	return at.currentToken.HasPermission(mask)
}

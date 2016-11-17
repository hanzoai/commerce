package token

import (
	"strings"
	"time"

	"appengine"

	"github.com/dgrijalva/jwt-go"

	"crowdstart.com/util/bit"
	"crowdstart.com/util/rand"
)

type Token struct {
	Secret []byte

	Name string

	// IssuedAt is the JWT "iat" param
	IssuedAt time.Time

	// EntityId is the JWT "sub" param
	EntityId string

	// Id is the JWT "jti" param
	Id string

	// Permissions is the JWT "bit" param
	Permissions bit.Field

	// Original token string
	TokenString string

	jwt *jwt.Token
}

func (t Token) String() string {
	signature, err := t.getJWT().SignedString(t.Secret)
	if err != nil {
		panic(err)
	}
	return signature
}

func (t Token) HasPermission(mask bit.Mask) bool {
	return t.Permissions.Has(mask)
}

func (t *Token) Get(field string) interface{} {
	jwt := t.getJWT()
	return jwt.Claims[field]
}

func (t *Token) Set(field string, value interface{}) *Token {
	jwt := t.getJWT()
	jwt.Claims[field] = value
	return t
}

func (t *Token) getJWT() *jwt.Token {
	if t.jwt != nil {
		return t.jwt
	}

	jwt := jwt.New(jwt.SigningMethodHS512)

	// jwt.Claims["name"] = t.Name
	// jwt.Claims["iat"] = t.IssuedAt.Unix()
	jwt.Claims["jti"] = t.Id
	jwt.Claims["sub"] = t.EntityId
	jwt.Claims["bit"] = int64(t.Permissions)

	// This sets the token to expire in a year
	// jwt.Claims["exp"] = t.IssuedAt.Add(time.Hour * 24.0 * 365).Unix()

	t.jwt = jwt

	return jwt
}

func (t *Token) Verify(ctx appengine.Context, secret []byte) (bool, error) {
	parts := strings.Split(t.TokenString, ".")

	if err := t.getJWT().Method.Verify(strings.Join(parts[0:2], "."), parts[2], secret); err != nil {
		return false, err
	}

	// Update secret on token
	t.Secret = secret

	return true, nil
}

func New(name string, subject string, permissions bit.Mask, secret []byte) *Token {
	tok := new(Token)
	tok.Id = rand.ShortId()
	tok.Secret = secret
	tok.EntityId = subject
	tok.IssuedAt = time.Now()
	tok.Name = name
	tok.Permissions = bit.Field(permissions)
	return tok
}

func FromString(accessToken string, secret []byte) (*Token, error) {
	tok := new(Token)
	tok.TokenString = accessToken

	// jwt.Parse takes a function that returns the secret used to validate
	// that we issued this accessToken using our secrets
	jwt, err := Parse(accessToken)
	if err != nil {
		return nil, err
	}

	// tok.Name = jwt.Claims["name"].(string)
	// tok.IssuedAt = time.Unix(int64(jwt.Claims["iat"].(float64)), 0)
	tok.Id = jwt.Claims["jti"].(string)
	tok.EntityId = jwt.Claims["sub"].(string)
	tok.Permissions = bit.Field(jwt.Claims["bit"].(int64))
	tok.jwt = jwt
	tok.Secret = secret

	return tok, nil
}

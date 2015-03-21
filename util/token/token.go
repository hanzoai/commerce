package token

import (
	"errors"
	"time"

	"github.com/dgrijalva/jwt-go"

	"crowdstart.io/util/bit"
)

type Token struct {
	Secret []byte

	Name string

	// IssuedAt is the JWT "iat" param
	IssuedAt time.Time

	// ModelId is the JWT "sub" param
	ModelId string

	// Id is the JWT "jti" param
	Id string

	// Permissions is the JWT "bit" param
	Permissions bit.Field
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

func (t *Token) getJWT() *jwt.Token {
	jwt := jwt.New(jwt.SigningMethodHS512)

	jwt.Claims["name"] = t.Name
	jwt.Claims["sub"] = t.ModelId
	jwt.Claims["iat"] = t.IssuedAt
	jwt.Claims["jti"] = t.Id
	jwt.Claims["bit"] = int64(t.Permissions)

	// This sets the token to expire in a year
	// jwt.Claims["exp"] = t.IssuedAt.Add(time.Hour * 24.0 * 365).Unix()

	return jwt
}

func New(name string, subject string, permissions bit.Mask, secret []byte) *Token {
	tok := new(Token)
	tok.Secret = secret
	tok.IssuedAt = time.Now()
	tok.Name = name
	tok.Permissions = bit.Field(permissions)
	return tok
}

func FromString(accessToken string, secret []byte) (*Token, error) {
	tok := new(Token)

	// jwt.Parse takes a function that returns the secret used to validate
	// that we issued this accessToken using our secrets
	jwt, err := jwt.Parse(accessToken, func(token *jwt.Token) (interface{}, error) {
		return secret, nil
	})

	if err != nil {
		return tok, err
	}

	if !jwt.Valid {
		return tok, errors.New("Not Valid")
	}

	tok.Id = jwt.Claims["jti"].(string)
	tok.IssuedAt = jwt.Claims["iat"].(time.Time)
	tok.ModelId = jwt.Claims["sub"].(string)
	tok.Name = jwt.Claims["name"].(string)
	tok.Permissions = jwt.Claims["bit"].(bit.Field)

	return tok, nil
}

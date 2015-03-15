package mixin

import (
	"errors"
	"strconv"
	"time"

	"crowdstart.io/util/rand"

	"github.com/dgrijalva/jwt-go"
)

// Error for expired jti's
var ErrorExpiredToken = errors.New("This token is expired.")

// AccessTokener is a mixin for securing objects with an AccessToken
type AccessTokener struct {
	Model model `json:"-" datastore:"-"`

	// Use IssuedAt as JWT "iat" param
	IssuedAt time.Time `json:"-"`
	// JWT secret
	SecretKey []byte `json:"-"`

	// UseTokenId as JWT "jti" param, randomly generate upon generating a new key to expire all existing keys
	TokenId string `json:"-"`
}

func (at *AccessTokener) GenerateAccessToken() (string, error) {
	// Generate a new TokenId to invalidate previous key
	at.TokenId = rand.ShortId()

	return at.accessToken()
}

func (at *AccessTokener) accessToken() (string, error) {
	token := jwt.New(jwt.SigningMethodHS512)

	// Use Key as JWT "iss" param
	token.Claims["iss"] = strconv.Itoa(int(at.Model.Key().IntID()))
	token.Claims["iat"] = at.IssuedAt
	token.Claims["jti"] = at.TokenId

	return token.SignedString(at.SecretKey)
}

func GetWithAccessToken(accessToken string, at *AccessTokener) error {
	m := at.Model
	t, err := jwt.Parse(accessToken, func(token *jwt.Token) (interface{}, error) {
		err2 := m.Get(token.Claims["iss"].(string))
		if err2 != nil {
			return nil, err2
		}

		if token.Claims["jti"].(string) != at.TokenId {
			return nil, ErrorExpiredToken
		}

		//log.Warn("iss %v, Key %v", token.Claims["iss"], at.SecretKey)
		return at.SecretKey, nil
	})

	if err != nil {
		return err
	}

	if !t.Valid {
		return errors.New("Not Valid")
	}

	return nil
}

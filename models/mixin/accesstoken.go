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

// AccessToken is a mixin for securing objects with an AccessToken
type AccessToken struct {
	// Model is a struct with a Model mixin
	Model model `json:"-" datastore:"-"`

	// Use IssuedAt as JWT "iat" param
	IssuedAt time.Time `json:"-"`
	// JWT secret
	SecretKey []byte `json:"-"`

	// UseTokenId as JWT "jti" param, randomly generate upon generating a new key to expire all existing keys
	TokenId string `json:"-"`
}

func (at *AccessToken) GenerateAccessToken() (string, error) {
	// Generate a new TokenId to invalidate previous key
	at.TokenId = rand.ShortId()

	return at.accessToken()
}

func (at *AccessToken) accessToken() (string, error) {
	token := jwt.New(jwt.SigningMethodHS512)

	// Use Key as JWT "iss" param
	token.Claims["iss"] = strconv.Itoa(int(at.Model.Key().IntID()))
	token.Claims["iat"] = at.IssuedAt
	token.Claims["jti"] = at.TokenId

	// This sets the token to expire in a year
	//token.Claims["exp"] = time.Now().Add(time.Hour * 24.0 * 365).Unix()

	return token.SignedString(at.SecretKey)
}

func (at *AccessToken) GetWithAccessToken(accessToken string) error {
	m := at.Model

	// jwt.Parse takes a function that returns the secret used to validate
	// that we issued this accessToken using our secrets
	t, err := jwt.Parse(accessToken, func(token *jwt.Token) (interface{}, error) {
		// Load the Model using the issuer ("iss")
		err2 := m.Get(token.Claims["iss"].(string))
		// If we can't load the Model, that means the metadata is stale
		if err2 != nil {
			return nil, err2
		}

		// If the jti mismatches, then the token is expired
		if at.TokenId != "" && at.TokenId != token.Claims["jti"].(string) {
			return nil, ErrorExpiredToken
		}

		// Return the Model's secret key to get the validity
		// of this token
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

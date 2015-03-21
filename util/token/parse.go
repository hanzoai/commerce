package token

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/dgrijalva/jwt-go"
)

var (
	MalformedToken           = errors.New("Token is malformed")
	InvalidNumberOfSegments  = errors.New("Token contains an invalid number of segments")
	UnavailableSigningMethod = errors.New("Token is unverifiable, signing method is unavailable")
	UnspecifiedSigningMethod = errors.New("Token is unverifiable, signing method is unspecified")
	TokenIsExpired           = errors.New("Token is expired")
	TokenIsNotValidYet       = errors.New("Token is not valid yet")
)

func Parse(tokenString string) (*jwt.Token, error) {
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return nil, InvalidNumberOfSegments
	}

	var err error
	token := &jwt.Token{Raw: tokenString}

	// parse Header
	var headerBytes []byte
	if headerBytes, err = jwt.DecodeSegment(parts[0]); err != nil {
		return token, MalformedToken
	}
	if err = json.Unmarshal(headerBytes, &token.Header); err != nil {
		return token, MalformedToken
	}

	// parse Claims
	var claimBytes []byte
	if claimBytes, err = jwt.DecodeSegment(parts[1]); err != nil {
		return token, MalformedToken
	}
	if err = json.Unmarshal(claimBytes, &token.Claims); err != nil {
		return token, MalformedToken
	}

	// Lookup signature method
	if method, ok := token.Header["alg"].(string); ok {
		if token.Method = jwt.GetSigningMethod(method); token.Method == nil {
			return token, UnavailableSigningMethod
		}
	} else {
		return token, UnspecifiedSigningMethod
	}

	// Check expiration times
	now := jwt.TimeFunc().Unix()
	if exp, ok := token.Claims["exp"].(float64); ok {
		if now > int64(exp) {
			return token, TokenIsExpired
		}
	}
	if nbf, ok := token.Claims["nbf"].(float64); ok {
		if now < int64(nbf) {
			return token, TokenIsNotValidYet
		}
	}

	return token, nil
}

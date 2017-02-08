package token

import (
	"encoding/json"
	"strings"

	"github.com/dgrijalva/jwt-go"
)

func parse(tokenString string) (*jwt.Token, error) {
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return nil, InvalidNumberOfSegments
	}

	var err error
	tok := &jwt.Token{Raw: tokenString}

	// parse Header
	var headerBytes []byte
	if headerBytes, err = jwt.DecodeSegment(parts[0]); err != nil {
		return tok, MalformedToken
	}
	if err = json.Unmarshal(headerBytes, &tok.Header); err != nil {
		return tok, MalformedToken
	}

	// parse Claims
	var claimBytes []byte
	if claimBytes, err = jwt.DecodeSegment(parts[1]); err != nil {
		return tok, MalformedToken
	}
	if err = json.Unmarshal(claimBytes, &tok.Claims); err != nil {
		return tok, MalformedToken
	}

	// Lookup signature method
	if method, ok := tok.Header["alg"].(string); ok {
		if tok.Method = jwt.GetSigningMethod(method); tok.Method == nil {
			return tok, UnavailableSigningMethod
		}
	} else {
		return tok, UnspecifiedSigningMethod
	}

	// Check expiration times
	now := jwt.TimeFunc().Unix()
	if exp, ok := tok.Claims["exp"].(float64); ok {
		if now > int64(exp) {
			return tok, TokenIsExpired
		}
	}
	if nbf, ok := tok.Claims["nbf"].(float64); ok {
		if now < int64(nbf) {
			return tok, TokenIsNotValidYet
		}
	}

	return tok, nil
}

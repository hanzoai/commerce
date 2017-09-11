package oauthtoken

import (
	"errors"

	"hanzo.io/util/jwt"
)

var (
	InvalidTokenType      = errors.New("Invalid token type")
	TokenOwnershipInvalid = errors.New("Token does not belong to this user")
	TokenRevoked          = errors.New("Token is revoked")
	TokenIsExpired        = jwt.TokenIsExpired
	TokenIsNotValidYet    = jwt.TokenIsNotValidYet
)

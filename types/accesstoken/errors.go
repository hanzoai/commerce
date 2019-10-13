package accesstoken

import (
	"hanzo.io/util/jwt"
)

var (
	TokenIsExpired     = jwt.TokenIsExpired
	TokenIsNotValidYet = jwt.TokenIsNotValidYet
)

package oauthtoken

import (
	"math"
	"time"

	"hanzo.io/models/mixin"

	"hanzo.io/util/bit"
	"hanzo.io/util/jwt"
	"hanzo.io/util/log"
	"hanzo.io/util/rand"
)

const (
	Algorithm = "HS256"
)

type Token struct {
	mixin.Model

	Claims Claims `json:"claims"`

	Name string `json:"name"`
	// In Hours
	AccessPeriod int64 `json:"accessPeriod"`
	Revoked      bool  `json:"revoked"`

	String string `json:"-" datastore:",noindex"`
}

func (t *Token) Defaults() {
	t.Claims = Claims{
		Type: Refresh,
		Claims: jwt.Claims{
			IssuedAt: time.Now().Unix(),
		},
	}
}

func (t *Token) Encode(secret []byte) (string, error) {
	if str, err := jwt.Encode(t.Claims, secret, Algorithm); err != nil {
		return str, err
	} else {
		t.String = str
		return str, nil
	}
}

func (t *Token) Decode(str string, secret []byte) error {
	return jwt.Decode(str, secret, Algorithm, &t.Claims)
}

func (t *Token) IsValid(nowUnix int64) error {
	if t.Revoked {
		return TokenRevoked
	}

	// Check for claims validity
	if t.Claims.BeforeExp(nowUnix) {
		return TokenIsExpired
	}
	if t.Claims.AfterNbf(nowUnix) {
		return TokenIsNotValidYet
	}

	return nil
}

func (t *Token) IssueRefreshToken(usrId string, secret []byte) (string, error) {
	now := time.Now()
	nowUnix := now.Unix()

	if err := t.IsValid(nowUnix); err != nil {
		return "", err
	}

	claims := t.Claims.Clone().(Claims)

	switch claims.Type {
	case Reference:
		claims.Issuer = t.Id()
		claims.JTI = rand.ShortId()
		claims.Type = Refresh
		claims.IssuedAt = nowUnix
		claims.ExpirationTime = now.Add(time.Duration(math.Max(1, float64(t.AccessPeriod))) * time.Hour).Unix()
		claims.Permissions = bit.Field(0)

		return jwt.Encode(claims, secret, Algorithm)
	default:
		return "", InvalidTokenType
	}
}

// Issues short term expiration token for site/cli/dashboard
func (t *Token) IssueAccessToken(usrId string, secret []byte) (string, error) {
	now := time.Now()
	nowUnix := now.Unix()

	if err := t.IsValid(nowUnix); err != nil {
		return "", err
	}

	claims := t.Claims.Clone().(Claims)

	switch claims.Type {
	case Reference:
		// Issues short term expiration token for cli/dashboard
		// Refresh tokens match the user with the UserId stored on the token before issueing

		claims.Type = Access
		if claims.UserId != usrId {
			return "", TokenOwnershipInvalid
		}

	case Api:
		// Issues short term expiration token for external sites
		// Site tokens should only have the originating organization assigned to it
		// UserId is added to the token when issued

		claims.Type = Customer
		claims.UserId = usrId

	default:
		return "", InvalidTokenType
	}

	claims.Issuer = t.Id()
	claims.JTI = rand.ShortId()
	claims.IssuedAt = nowUnix
	claims.ExpirationTime = now.Add(time.Duration(math.Max(1, float64(t.AccessPeriod))) * time.Hour).Unix()
	log.Debug("Issuing Claims %v", claims.JSON())
	return jwt.Encode(claims, secret, Algorithm)
}

func (t *Token) Revoke() {
	t.Revoked = true
	t.MustUpdate()
}

// Reference Based Tokens
func IsReference(claims Claims) bool {
	return claims.Type == Reference && claims.UserId != "" && claims.OrganizationName != "" && claims.AppId == ""
}

func IsRefresh(claims Claims) bool {
	return claims.Type == Refresh && claims.UserId != "" && claims.OrganizationName != "" && claims.AppId == ""
}

func IsAccess(claims Claims) bool {
	return claims.Type == Access && claims.UserId != "" && claims.OrganizationName != "" && claims.AppId == ""
}

// API Based Tokens
func IsApi(claims Claims) bool {
	return claims.Type == Api && claims.UserId == "" && claims.OrganizationName != "" && claims.AppId != ""
}

func IsCustomer(claims Claims) bool {
	return claims.Type == Customer && claims.UserId != "" && claims.OrganizationName != "" && claims.AppId != ""
}

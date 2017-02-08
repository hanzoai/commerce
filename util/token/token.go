package token

import (
	"strings"
	"time"

	jwt "github.com/dgrijalva/jwt-go"

	"hanzo.io/util/bit"
	"hanzo.io/util/log"
	"hanzo.io/util/timeutil"
)

// Read optimized Token struct
type Token struct {
	// These are the standard fields we expect in the jwt claims

	// Type is the JWT Custom "typ" param
	Type string `json:"type"`

	// IssuedAt is the JWT "iat" param
	IssuedAt time.Time `json:"issuedAt"`

	// ExpiredAt is the JWT "exp" param
	ExpiredAt time.Time `json:"expiredAt"`

	// Jti is the JWT "jti" param (optional)
	Jti string `json:"jti"`

	// Permissions is the JWT Custom "bit" param
	Permissions bit.Field `json:"permissions"`

	// Scopes is the JWT Custom "scopes" param
	Scopes string `json:"scopes"`

	// Organization is the Organization/Namespace and the JWT Custom "org" param
	Organization string `json:"organization"`

	// UserId is the JWT Custom "usr" param
	UserId string `json:"userId"`

	// teams is the JWT Custom "teams" param
	//Teams []string `json:"-"`

	// Other Fields

	// Cached last issued jwt token
	TokenString string `json:"-"`

	// Temporarily cached secret for decoding
	secret []byte     `json:"-" datastore:"-"`
	tok    *jwt.Token `json:"-" datastore:"-"`
}

// Typically this is not called directly, just access TokenString
func (t Token) String() string {
	if t.tok == nil {
		t.UpdateJWTFromToken()
	}

	tStr, err := t.tok.SignedString(t.secret)
	if err != nil {
		panic(err)
	}

	return tStr
}

func (t Token) HasPermission(mask bit.Mask) bool {
	return t.Permissions.Has(mask)
}

func (t *Token) Get(field string) interface{} {
	value := t.tok.Claims[field]

	switch field {
	case "iat", "exp":
		return time.Unix(value.(int64), 0)
	case "scopes":
		return strings.Join(value.([]string), " ")
	case "bit":
		return value.(bit.Field)
	}

	return value
}

func (t *Token) Set(field string, value interface{}) *Token {
	switch field {
	case "iat", "exp":
		value = value.(time.Time).Unix()
	case "scopes":
		value = strings.Split(value.(string), " ")
	case "bit":
		value = value.(bit.Field)
	}

	t.tok.Claims[field] = value

	t.TokenString = t.UpdateTokenFromJWT(field).String()
	return t
}

func (t *Token) Verify() bool {
	parts := strings.Split(t.TokenString, ".")

	if err := t.tok.Method.Verify(strings.Join(parts[0:2], "."), parts[2], t.secret); err != nil {
		log.Error("Token Verification Error %v", err)
		return false
	}

	return true
}

func (t *Token) UpdateTokenFromJWT(field string) *Token {
	if t.tok == nil {
		return t.UpdateJWTFromToken()
	}

	switch field {
	// Do everything!
	case "*":
		t.Jti = t.tok.Claims["jti"].(string)
		t.Type = t.tok.Claims["typ"].(string)
		unix, ok := t.tok.Claims["iat"].(int64)
		if !ok {
			unix = int64(t.tok.Claims["iat"].(float64))
		}
		if unix > 0 {
			t.IssuedAt = time.Unix(unix, 0)
		}
		unix, ok = t.tok.Claims["exp"].(int64)
		if !ok {
			unix = int64(t.tok.Claims["exp"].(float64))
		}
		if unix > 0 {
			t.ExpiredAt = time.Unix(unix, 0)
		}
		t.Permissions, ok = t.tok.Claims["bit"].(bit.Field)
		if !ok {
			t.Permissions = bit.Field(t.tok.Claims["bit"].(float64))
		}
		s, ok := t.tok.Claims["scopes"].([]string)
		if s == nil {
			s = make([]string, 0)
		}
		t.Scopes = strings.Join(s, " ")
		t.Organization = t.tok.Claims["org"].(string)
		t.UserId = t.tok.Claims["usr"].(string)
	case "jti":
		t.Jti = t.tok.Claims["jti"].(string)
	case "typ":
		t.Type = t.tok.Claims["typ"].(string)
	case "iat":
		unix, ok := t.tok.Claims["iat"].(int64)
		if !ok {
			unix = int64(t.tok.Claims["iat"].(float64))
		}
		if unix > 0 {
			t.IssuedAt = time.Unix(unix, 0)
		}
	case "exp":
		unix, ok := t.tok.Claims["exp"].(int64)
		if !ok {
			unix = int64(t.tok.Claims["exp"].(float64))
		}
		if unix > 0 {
			t.ExpiredAt = time.Unix(unix, 0)
		}
	case "bit":
		perm, ok := t.tok.Claims["bit"].(bit.Field)
		t.Permissions = perm
		if !ok {
			t.Permissions = bit.Field(t.tok.Claims["bit"].(float64))
		}
	case "scopes":
		s := t.tok.Claims["scopes"].([]string)
		if s == nil {
			s = make([]string, 0)
		}
		t.Scopes = strings.Join(s, " ")
	case "org":
		t.Organization = t.tok.Claims["org"].(string)
	case "usr":
		t.UserId = t.tok.Claims["usr"].(string)
	default:
		log.Info("No struct field associated with JWT Claim '%v'", field)
	}
	return t
}

// Optimize for read
func (t *Token) UpdateJWTFromToken() *Token {
	if t.tok == nil {
		t.tok = jwt.New(jwt.SigningMethodHS512)
	}

	t.tok.Claims["typ"] = t.Type
	t.tok.Claims["iat"] = t.IssuedAt.Unix()

	if !timeutil.IsZero(t.ExpiredAt) {
		t.tok.Claims["exp"] = t.ExpiredAt.Unix()
	} else {
		delete(t.tok.Claims, "exp")
	}

	if t.Jti != "" {
		t.tok.Claims["jti"] = t.Jti
	} else {
		delete(t.tok.Claims, "jti")
	}

	t.tok.Claims["bit"] = t.Permissions

	if t.Organization != "" {
		t.tok.Claims["org"] = t.Organization
	} else {
		delete(t.tok.Claims, "org")
	}

	if t.UserId != "" {
		t.tok.Claims["usr"] = t.UserId
	} else {
		delete(t.tok.Claims, "usr")
	}

	if len(t.Scopes) > 0 {
		t.tok.Claims["scopes"] = strings.Split(t.Scopes, " ")
	} else {
		delete(t.tok.Claims, "scopes")
	}

	// This sets the token to expire in a year
	// jwt.Claims["exp"] = t.IssuedAt.Add(time.Hour * 24.0 * 365).Unix()

	return t
}

func (t *Token) SetSecret(secret []byte) {
	t.secret = secret
}

func (t *Token) Clone() (*Token, error) {
	return Parse(t.TokenString, t.secret)
}

func New(typ string, permissions bit.Mask, secret []byte) *Token {
	tok := new(Token)
	tok.Type = typ
	tok.IssuedAt = time.Now()
	tok.Permissions = bit.Field(permissions)
	tok.secret = secret
	tok.UpdateJWTFromToken()
	tok.TokenString = tok.String()
	return tok
}

func Parse(tStr string, secret []byte) (*Token, error) {
	tok := new(Token)
	tok.secret = secret
	tok.TokenString = tStr

	// jwt.Parse takes a function that returns the secret used to validate
	// that we issued this accessToken using our secrets
	t, err := parse(tok.TokenString)
	if err != nil {
		return nil, err
	}

	tok.tok = t
	if !tok.Verify() {
		return nil, TokenCouldNotBeDecoded
	}

	return tok.UpdateTokenFromJWT("*"), nil
}

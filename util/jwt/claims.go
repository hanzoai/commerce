package jwt

import "encoding/json"

type Claimable interface {
	Validate() error
	AfterNbf(int64) bool
	BeforeExp(int64) bool
	Clone() Claimable
}

type Claims struct {
	IssuedAt       int64 `json:"iat,omitempty"`
	ExpirationTime int64 `json:"exp,omitempty"`
	NotBefore      int64 `json:"nbf,omitempty"`

	Issuer   string `json:"iss,omitempty"`
	Subject  string `json:"sub,omitempty"`
	Audience string `json:"aud,omitempty"`
	JTI      string `json:"jti,omitempty"`

	ValidateFn func() error `json:"-" datastore:"-"`
}

func (c Claims) Validate() error {
	if c.ValidateFn == nil {
		return nil
	}

	return c.ValidateFn()
}

func (c Claims) AfterNbf(now int64) bool {
	return c.NotBefore != 0 && now < c.NotBefore
}

func (c Claims) BeforeExp(now int64) bool {
	return c.ExpirationTime != 0 && now > c.ExpirationTime
}

func (c Claims) Clone() Claimable {
	return c
}

func (c Claims) JSON() string {
	j, _ := json.Marshal(c)
	return string(j)
}

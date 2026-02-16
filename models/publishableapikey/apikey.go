package publishableapikey

import (
	"time"

	"github.com/hanzoai/commerce/models/mixin"
)

type KeyType string

const (
	Publishable KeyType = "publishable"
	Secret      KeyType = "secret"
)

type PublishableApiKey struct {
	mixin.Model

	Title      string     `json:"title"`
	Type       KeyType    `json:"type"`
	Token      string     `json:"-"`             // never expose in JSON
	Salt       string     `json:"-"`
	Redacted   string     `json:"redacted"`      // last 4 chars
	LastUsedAt *time.Time `json:"lastUsedAt,omitempty"`
	RevokedAt  *time.Time `json:"revokedAt,omitempty"`
	RevokedBy  string     `json:"revokedBy"`
}

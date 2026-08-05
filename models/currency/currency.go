// Package currency is the currency entity domain: the reference table of the
// currencies a store accepts, addressable by ISO-4217 Code. It is a global
// (default-namespace) reference set — the single source the store/settings and
// product/price currency pickers read, replacing a hardcoded array. The bare
// value type (usd/eur/…) and its symbol/decimal conventions still live in
// models/types/currency; this entity carries the editable, seedable rows.
package currency

import (
	"strings"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/orm"
)

func init() { orm.Register[Currency]("currency") }

// Currency is one accepted currency, keyed by its lowercase ISO-4217 Code
// (unique). DecimalDigits is the minor-unit exponent (2 for usd, 0 for jpy);
// IncludesTax marks currencies whose displayed prices are tax-inclusive.
type Currency struct {
	mixin.Model[Currency]

	// Code is the lowercase ISO-4217 code (e.g. "usd"). Unique reference key.
	Code string `json:"code"`

	// Symbol is the display glyph (e.g. "$", "€", "¥").
	Symbol string `json:"symbol"`

	// Name is the human-facing currency name (e.g. "US Dollar").
	Name string `json:"name"`

	// DecimalDigits is the number of minor-unit decimal places (2 for usd, 0 for jpy).
	DecimalDigits int `json:"decimalDigits"`

	// IncludesTax marks prices in this currency as tax-inclusive.
	IncludesTax bool `json:"includesTax"`
}

// Normalize lowercases the code before save so lookups are case-insensitive and
// the uniqueness key is canonical.
func (c *Currency) Normalize() { c.Code = strings.ToLower(strings.TrimSpace(c.Code)) }

func New(db *datastore.Datastore) *Currency {
	c := new(Currency)
	c.Init(db)
	return c
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query("currency")
}

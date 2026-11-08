// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

// Package customerprice is the price one customer pays for one plan, every
// period, as staff agreed it: "this account pays $195 a month for Unlimited".
//
// One row per (subject, plan, interval), its id derived from them (ID), so
// setting a price again replaces it rather than stacking a second one. The card
// sale charges it and the subscription it opens carries it, so every renewal
// bills it; the invoice shows the plan at its list price and the difference as
// the customer's discount.
package customerprice

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/orm"
)

func init() {
	orm.Register[CustomerPrice]("customer-price", orm.WithStringKey[CustomerPrice]())
}

// CustomerPrice is what Subject pays for one seat of Plan each Interval.
type CustomerPrice struct {
	mixin.Model[CustomerPrice]
	// Subject is the billing subject: the org's own account, or a member's wallet.
	Subject string `json:"subject"`
	// Plan is the plan's slug.
	Plan string `json:"plan"`
	// Interval is the period the price is for: "month" or "year".
	Interval string `json:"interval"`
	// Cents is one seat's price for one period, below the plan's list price.
	Cents int64 `json:"cents"`
	// Note is why, as staff wrote it.
	Note string `json:"note,omitempty"`
}

func (p *CustomerPrice) Load(ps []datastore.Property) error {
	return datastore.LoadStruct(p, ps)
}

func (p *CustomerPrice) Save() ([]datastore.Property, error) {
	return datastore.SaveStruct(p)
}

// ID is the storage id of subject's price for plan at interval.
func ID(subject, plan, interval string) string {
	sum := sha256.Sum256([]byte(subject + "\x00" + plan + "\x00" + interval))
	return "cpx_" + hex.EncodeToString(sum[:16])
}

func New(db *datastore.Datastore) *CustomerPrice {
	p := new(CustomerPrice)
	p.Init(db)
	return p
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query("customer-price")
}

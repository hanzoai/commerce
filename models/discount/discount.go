package discount

import (
	"time"

	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"
	"crowdstart.com/util/log"
	"crowdstart.com/util/timeutil"
)

var IgnoreFieldMismatch = datastore.IgnoreFieldMismatch

type Type string

const (
	Flat         Type = "flat"
	Percent           = "percent"
	FreeShipping      = "free-shipping"
	FreeItem          = "free-item"
)

var Types = []Type{Flat, Percent, FreeShipping}

type Discount struct {
	mixin.Model

	Name string `json:"name"`

	// Possible values: flat, percent, free_shipping.
	Type Type `json:"type"`

	// Range in which discount is valid
	StartDate time.Time `json:"startDate"`
	EndDate   time.Time `json:"endDate"`

	// Possible values: order, product.
	Filter string `json:"filter"`

	// Product id for product-specific discount.
	ProductId string `json:"productId,omitempty"`

	// Whether discount is enabled.
	Enabled bool `json:"enabled"`

	// Discount amount. $5 should be 500 (prices in basic currency unit, like cents). 10% should be 10.
	Amount int `json:"amount"`

	// Free product with enabled
	FreeProductId string `json:"freeProductId"`
	FreeVariantId string `json:"freeVariantId"`
	FreeQuantity  int    `json:"freeQuantity"`
}

func (d Discount) ValidFor(t time.Time) bool {
	ctx := d.Context()
	if !d.Enabled {
		log.Warn("Discount Not Enabled", ctx)
		return false // currently active, no need to check?
	}

	if !timeutil.IsZero(d.StartDate) && d.StartDate.After(t) {
		log.Warn("Discount not yet Usable: %v > %v", d.StartDate.Unix(), t, ctx)
		return false
	}

	if !timeutil.IsZero(d.EndDate) && d.EndDate.Before(t) {
		log.Warn("Discount is Expired: %v < %v", d.EndDate.Unix(), t, ctx)
		return false
	}

	return true
}

func (d Discount) ItemId() string {
	if d.ProductId != "" {
		return d.ProductId
	}
	if d.FreeProductId != "" {
		return d.FreeProductId
	}
	if d.FreeVariantId != "" {
		return d.FreeProductId
	}
	return ""
}

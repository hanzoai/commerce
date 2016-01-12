package coupon

import (
	"strings"
	"time"

	aeds "appengine/datastore"

	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"
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

type Coupon struct {
	mixin.Model

	Name string `json:"name"`

	// Possible values: flat, percent, free_shipping.
	Type Type `json:"type"`

	// Coupon code (must be unique).
	Code string `json:"code"`

	CampaignId string `json:"campaignId,omitempty"`

	// Range in which coupon is valid
	StartDate time.Time `json:"startDate"`
	EndDate   time.Time `json:"endDate"`

	// Possible values: order, product.
	Filter string `json:"filter"`

	// Apply once or to every time
	Once bool `json:"once"`

	// Product id for product-specific coupons.
	ProductId string `json:"productId,omitempty"`

	// Whether coupon is valid.
	Enabled bool `json:"enabled"`

	// Coupon amount. $5 should be 500 (prices in basic currency unit, like cents). 10% should be 10.
	Amount int `json:"amount"`

	// Number of times coupon was redeemed.
	Used int `json:"used"`

	// Free product with coupon
	FreeProductId string `json:"freeProductId"`
	FreeVariantId string `json:"freeVariantId"`
	FreeQuantity  int    `json:"freeQuantity"`

	// List of buyer email addresses who have redeemed coupon.
	// Buyers []string `json:"buyers"`
}

func (co *Coupon) Load(c <-chan aeds.Property) (err error) {
	// Load supported properties
	if err = IgnoreFieldMismatch(aeds.LoadStruct(co, c)); err != nil {
		return err
	}

	return err
}

func (co *Coupon) Save(c chan<- aeds.Property) (err error) {

	co.Code = strings.ToUpper(co.Code)

	// Save properties
	return IgnoreFieldMismatch(aeds.SaveStruct(co, c))
}

func (c Coupon) ValidFor(t time.Time) bool {
	if c.Enabled {
		return true // currently active, no need to check?
	}

	if c.StartDate.Before(t) && c.EndDate.After(t) {
		return true
	}

	return false
}

func (c Coupon) ItemId() string {
	if c.ProductId != "" {
		return c.ProductId
	}
	if c.FreeProductId != "" {
		return c.FreeProductId
	}
	if c.FreeVariantId != "" {
		return c.FreeProductId
	}
	return ""
}

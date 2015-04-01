package coupon

import (
	"time"

	"crowdstart.io/datastore"
	"crowdstart.io/models/mixin"
	"crowdstart.io/util/val"
)

type CouponType string

const (
	Flat         CouponType = "flat"
	Percent                 = "percent"
	FreeShipping            = "free-shipping"
)

type Coupon struct {
	mixin.Model

	// Possible values: flat, percent, free_shipping.
	Type CouponType `json:"type"`

	// Coupon code (must be unique).
	Code string `json:"code"`

	CampaignId string `json:"campaignId"`

	// Range in which coupon is valid
	StartDate time.Time `json:"startDate"`
	EndDate   time.Time `json:"endDate"`

	// Possible values: order, product.
	Filter string `json:"filter"`

	// Apply once or to every time
	Once bool `json:"once"`

	// Product id for product-specific coupons.
	ProductId string `json:"productId"`

	// Whether coupon is valid.
	Enabled bool `json:"enabled"`

	// Coupon amount. $5 should be 500 (prices in cents). 10% should be 10.
	Amount int `json:"amount"`

	// Number of times coupon was redeemed.
	Used int `json:"used"`

	// List of buyer email addresses who have redeemed coupon.
	//Buyers []string `json:"buyers"`
}

func New(db *datastore.Datastore) *Coupon {
	c := new(Coupon)
	c.Model = mixin.Model{Db: db, Entity: c}
	//c.Buyers = make([]string, 0)
	return c
}

func (c Coupon) Kind() string {
	return "coupon"
}

func (c *Coupon) Validator() *val.Validator {
	return val.New(c)
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}

package coupon

import (
	"time"

	"crowdstart.io/datastore"
	"crowdstart.io/models/mixin"
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
	Type CouponType

	// Coupon code (must be unique).
	Code string

	CampaignId string

	CreatedAt time.Time
	UpdatedAt time.Time

	// Range in which coupon is valid
	StartDate time.Time
	EndDate   time.Time

	// Possible values: order, product.
	Filter string

	// Apply once or to every time
	Once bool

	// Product id for product-specific coupons.
	ProductId string

	// Whether coupon is valid.
	Enabled bool

	// Coupon amount. $5 should be 500 (prices in cents). 10% should be 10.
	Amount int

	// Number of times coupon was redeemed.
	Used int

	// List of buyer email addresses who have redeemed coupon.
	Buyers []string
}

func New(db *datastore.Datastore) *Coupon {
	c := new(Coupon)
	c.Model = mixin.Model{Db: db, Entity: c}
	return c
}

func (c Coupon) Kind() string {
	return "coupon2"
}

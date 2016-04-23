package coupon

import (
	"time"

	"crowdstart.com/datastore"
)

type Redemption struct {
	// Time coupon was used
	CreatedAt time.Time `json:"code"`

	// Coupon code or dynamic code
	Code string `json:"code"`
}

func (c Coupon) SaveRedemption() error {
	db := datastore.New(c.Context())
	key := db.KeyFromId("redemption", c.Code())
	_, err := db.Put(key, &Redemption{time.Now(), c.Code()})
	return err
}

func (c Coupon) Redemptions() int {
	db := datastore.New(c.Context())
	count, _ := db.Query("redemption").Filter("Code=", c.Code()).Count()
	return count
}

func (c Coupon) Redeemable() bool {
	if !c.Enabled {
		return false
	}

	// Unlimited coupon usage if limit is set less than 1
	if c.Limit < 1 {
		return true
	}

	if c.Redemptions() >= c.Limit {
		return false
	}

	return true
}

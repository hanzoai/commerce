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

func (c Coupon) Redemptions(code string) int {
	db := datastore.New(c.Context())

	// If code is missing this is a normal coupon
	if code == "" {
		code = c.Code
	}

	return db.Query("redemption").Filter("Code=", code).Count()
}

func (c Coupon) SaveRedemptions(code string) err {
	db := datastore.New(c.Context())
	key := db.KeyFromId("redemption", code)
	return db.Put(key, &Redemption{time.Now(), code})
}

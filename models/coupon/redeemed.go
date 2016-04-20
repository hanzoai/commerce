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

func (c Coupon) Redemptions(code string) (int, error) {
	db := datastore.New(c.Context())

	// If code is missing this is a normal coupon
	if code == "" {
		code = c.Code
	}

	return db.Query("redemption").Filter("Code=", code).Count()
}

func (c Coupon) SaveRedemption(code string) error {
	db := datastore.New(c.Context())
	key := db.KeyFromId("redemption", code)
	_, err := db.Put(key, &Redemption{time.Now(), code})
	return err
}

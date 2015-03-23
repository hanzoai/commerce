package fixtures

import (
	"time"

	"github.com/gin-gonic/gin"

	"crowdstart.io/models2/coupon"
)

const Month = time.Hour * 24 * 30

func Coupon(c *gin.Context) *coupon.Coupon {
	db := getDb(c)

	coupon := coupon.New(db)
	coupon.Type = "flat"
	coupon.Code = "such-coupon"
	now := time.Now()
	coupon.StartDate = now
	coupon.EndDate = now.Add(Month)
	coupon.Once = true
	coupon.Enabled = true
	coupon.Amount = 5

	coupon.MustPut()
	return coupon
}

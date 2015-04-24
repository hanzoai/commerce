package fixtures

import (
	"time"

	"github.com/gin-gonic/gin"

	"crowdstart.io/models/coupon"
)

const Month = time.Hour * 24 * 30

var Coupon = New("coupon", func(c *gin.Context) *coupon.Coupon {
	db := getNamespaceDb(c)

	coupon := coupon.New(db)
	coupon.Code = "such-coupon"
	coupon.GetOrCreate("Code=", coupon.Code)
	coupon.Name = "Such Coupon"
	coupon.Type = "flat"
	now := time.Now()
	coupon.StartDate = now
	coupon.EndDate = now.Add(Month)
	coupon.Once = true
	coupon.Enabled = true
	coupon.Amount = 5

	coupon.MustPut()
	return coupon
})

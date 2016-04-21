package fixtures

import (
	"time"

	"github.com/gin-gonic/gin"

	"crowdstart.com/models/coupon"
)

const Month = time.Hour * 24 * 30

var Coupon = New("coupon", func(c *gin.Context) *coupon.Coupon {
	db := getNamespaceDb(c)

	now := time.Now()

	p := Product(c)

	cpn := coupon.New(db)
	cpn.Code_ = "sad-coupon"
	cpn.GetOrCreate("Code=", cpn.Code)
	cpn.Name = "Sad Coupon"
	cpn.Type = "flat"
	cpn.StartDate = now
	cpn.EndDate = now.Add(Month)
	cpn.Once = true
	cpn.Enabled = true
	cpn.Amount = 500
	cpn.ProductId = p.Id()

	cpn.MustPut()

	cpn = coupon.New(db)
	cpn.Code_ = "such-coupon"
	cpn.GetOrCreate("Code=", cpn.Code)
	cpn.Name = "Such Coupon"
	cpn.Type = "flat"
	cpn.StartDate = now
	cpn.EndDate = now.Add(Month)
	cpn.Once = true
	cpn.Enabled = true
	cpn.Amount = 500

	cpn.MustPut()

	cpn = coupon.New(db)
	cpn.Code_ = "FREE-DOGE"
	cpn.GetOrCreate("Code=", cpn.Code)
	cpn.Name = "Free DogeShirt"
	cpn.Type = "free-item"
	cpn.StartDate = now
	cpn.EndDate = now.Add(Month)
	cpn.Once = true
	cpn.Enabled = true
	cpn.FreeProductId = "doge-shirt"
	cpn.FreeQuantity = 1
	cpn.MustPut()
	return cpn
})

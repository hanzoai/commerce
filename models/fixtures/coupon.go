package fixtures

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"hanzo.io/models/coupon"
)

const Month = time.Hour * 24 * 30

var Coupon = New("coupon", func(c *gin.Context) *coupon.Coupon {
	db := getNamespaceDb(c)

	now := time.Now()

	p := Product(c)

	cpn := coupon.New(db)
	cpn.Code_ = strings.ToUpper("sad-coupon")
	cpn.GetOrCreate("Code=", cpn.Code_)
	cpn.Name = "Sad Coupon"
	cpn.Type = "flat"
	cpn.EndDate = now.Add(Month)
	cpn.Once = true
	cpn.Enabled = true
	cpn.Amount = 500
	cpn.ProductId = p.Id()

	cpn.MustPut()

	cpn = coupon.New(db)
	cpn.Code_ = strings.ToUpper("such-coupon")
	cpn.GetOrCreate("Code=", cpn.Code_)
	cpn.Name = "Such Coupon"
	cpn.Type = "flat"
	cpn.EndDate = now.Add(Month)
	cpn.Once = true
	cpn.Enabled = true
	cpn.Amount = 500

	cpn.MustPut()

	cpn = coupon.New(db)
	cpn.Code_ = strings.ToUpper("FREE-DOGE")
	cpn.GetOrCreate("Code=", cpn.Code_)
	cpn.Name = "Free DogeShirt"
	cpn.Type = "free-item"
	cpn.EndDate = now.Add(Month)
	cpn.Once = true
	cpn.Enabled = true
	cpn.FreeProductId = "doge-shirt"
	cpn.FreeQuantity = 1

	cpn.MustPut()

	cpn = coupon.New(db)
	cpn.Code_ = strings.ToUpper("NO-DOGE-LEFT-BEHIND")
	cpn.GetOrCreate("Code=", cpn.Code_)
	cpn.Dynamic = true
	cpn.Limit = 1
	cpn.Name = "Free DogeShirt"
	cpn.Type = "free-item"
	cpn.EndDate = now.Add(Month)
	cpn.Once = true
	cpn.Enabled = true
	cpn.FreeProductId = "doge-shirt"
	cpn.FreeQuantity = 1

	cpn.MustPut()

	return cpn
})

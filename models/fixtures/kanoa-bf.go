package fixtures

import (
	"time"

	"github.com/gin-gonic/gin"

	"crowdstart.com/datastore"
	"crowdstart.com/models/coupon"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/product"
	"crowdstart.com/models/types/currency"
)

var _ = New("kanoa-bf", func(c *gin.Context) *organization.Organization {
	db := datastore.New(c)

	org := organization.New(db)
	org.Name = "kanoa"
	org.GetOrCreate("Name=", org.Name)

	nsCtx := org.Namespace(db.Context)
	db = datastore.New(nsCtx)

	// Free Cap Product
	prod := product.New(db)
	prod.Slug = "black-friday-cap"
	prod.GetOrCreate("Slug=", prod.Slug)
	prod.Name = "Cap"
	prod.Price = 0
	prod.Currency = currency.USD
	prod.MustPut()

	now := time.Now()

	// Black Friday Coupon
	cpn := coupon.New(db)
	cpn.Code = "BLACKANDBLUE"
	cpn.GetOrCreate("Code=", cpn.Code)
	cpn.Name = "Black Friday Coupon"
	cpn.Type = "free-item"
	cpn.StartDate = now
	cpn.Enabled = true
	cpn.FreeProductId = prod.Id()
	cpn.FreeQuantity = 1
	cpn.MustPut()

	return org
})

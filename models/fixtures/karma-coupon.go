package fixtures

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/models/organization"

	"hanzo.io/models/coupon"
)

var _ = New("karma-coupon", func(c *gin.Context) *coupon.Coupon {
	db := datastore.New(c)

	org := organization.New(db)
	org.Query().Filter("Name=", "karma").Get()

	nsdb := datastore.New(org.Namespaced(db.Context))

	now := time.Now()

	cpn := coupon.New(nsdb)
	cpn.Code_ = strings.ToUpper("DAYNA")
	cpn.GetOrCreate("Code=", cpn.Code_)
	cpn.Name = "Danya's Code"
	cpn.Type = "percent"
	cpn.StartDate = now
	cpn.Once = false
	cpn.Enabled = true
	cpn.Amount = 20

	cpn.MustPut()

	return cpn
})

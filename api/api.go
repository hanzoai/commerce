package api

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/models2/campaign"
	"crowdstart.io/models2/coupon"
	"crowdstart.io/models2/organization"
	"crowdstart.io/models2/product"
	"crowdstart.io/models2/token"
	"crowdstart.io/models2/user"
	"crowdstart.io/models2/variant"
	"crowdstart.io/util/rest"
	"crowdstart.io/util/router"
)

func init() {
	router := router.New("api")

	// Redirect root
	router.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	rest.New(campaign.Campaign{}).Route(router)
	rest.New(coupon.Coupon{}).Route(router)
	rest.New(organization.Organization{}).Route(router)
	rest.New(product.Product{}).Route(router)
	rest.New(token.Token{}).Route(router)
	rest.New(user.User{}).Route(router)
	rest.New(variant.Variant{}).Route(router)
}

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

	rest.Restify(router, new(campaign.Campaign))
	rest.Restify(router, new(coupon.Coupon))
	rest.Restify(router, new(organization.Organization))
	rest.Restify(router, new(product.Product))
	rest.Restify(router, new(token.Token))
	rest.Restify(router, new(user.User))
	rest.Restify(router, new(variant.Variant))
}

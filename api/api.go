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

	// Generate a new access token
	router.GET("/authorize/:id", func(c *gin.Context) {
		id := c.Params.ByName("id")

		query := c.Request.URL.Query()
		email := query.Get("email")
		password := query.Get("password")

		authorize(c, id, email, password)
	})

	router.POST("/authorize/:id", func(c *gin.Context) {
		id := c.Params.ByName("id")

		email := c.Request.Form.Get("email")
		password := c.Request.Form.Get("password")

		authorize(c, id, email, password)
	})

	// Namespaced by default
	rest.New(campaign.Campaign{}).Route(router)
	rest.New(coupon.Coupon{}).Route(router)
	rest.New(token.Token{}).Route(router)
	rest.New(user.User{}).Route(router)
	rest.New(organization.Organization{}).Route(router)
	rest.New(product.Product{}).Route(router)
	rest.New(variant.Variant{}).Route(router)
}

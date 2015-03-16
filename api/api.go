package api

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/models/mixin"
	"crowdstart.io/models2/campaign"
	"crowdstart.io/models2/collection"
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

	// Entities with automatic RESTful API
	entities := []mixin.Entity{
		campaign.Campaign{},
		coupon.Coupon{},
		collection.Collection{},
		organization.Organization{},
		product.Product{},
		token.Token{},
		user.User{},
		variant.Variant{},
	}

	// Redirect root
	router.GET("/", rest.DebugIndex(entities), func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// Access token routes
	router.GET("/access-token/:id", func(c *gin.Context) {
		id := c.Params.ByName("id")

		query := c.Request.URL.Query()
		email := query.Get("email")
		password := query.Get("password")

		getAccessToken(c, id, email, password)
	})

	router.POST("/access-token/:id", func(c *gin.Context) {
		id := c.Params.ByName("id")

		email := c.Request.Form.Get("email")
		password := c.Request.Form.Get("password")

		getAccessToken(c, id, email, password)
	})

	// Authorization routes
	router.POST("/checkout", checkout)
	router.POST("/authorize", authorize)
	router.POST("/capture", capture)

	// Setup API routes
	for _, entity := range entities {
		rest.New(entity).Route(router)
	}
}

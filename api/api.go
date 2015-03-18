package api

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/api/payment"
	"crowdstart.io/middleware"
	"crowdstart.io/models/mixin"
	"crowdstart.io/models2/campaign"
	"crowdstart.io/models2/collection"
	"crowdstart.io/models2/coupon"
	"crowdstart.io/models2/order"
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

	// Redirect naked, v1
	router.GET("/", func(c *gin.Context) {
		c.Redirect(301, "http://www.crowdstart.com/docs")
	})

	router.GET("/v1/", func(c *gin.Context) {
		c.Redirect(301, "http://www.crowdstart.com/docs")
	})

	v2 := router.New("api").Group("/v2/")

	// Entities with automatic RESTful API
	entities := []mixin.Entity{
		campaign.Campaign{},
		coupon.Coupon{},
		collection.Collection{},
		organization.Organization{},
		product.Product{},
		order.Order{},
		token.Token{},
		user.User{},
		variant.Variant{},
	}

	// Redirect root
	v2.GET("/", rest.DebugIndex(entities), func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// Access token routes
	v2.GET("/access/:id", func(c *gin.Context) {
		id := c.Params.ByName("id")

		query := c.Request.URL.Query()
		email := query.Get("email")
		password := query.Get("password")

		getAccessToken(c, id, email, password)
	})

	v2.POST("/access/:id", func(c *gin.Context) {
		id := c.Params.ByName("id")

		email := c.Request.Form.Get("email")
		password := c.Request.Form.Get("password")

		getAccessToken(c, id, email, password)
	})

	v2.DELETE("/access", middleware.TokenRequired(), func(c *gin.Context) {
		deleteAccessToken(c)
	})

	// Authorization routes
	// One Step Payments
	v2.POST("/charge", middleware.TokenRequired(), payment.Charge)
	v2.POST("/order/:id/charge", middleware.TokenRequired(), payment.Charge)

	// Two Step Payments - "Auth & Capture"
	v2.POST("/authorize", middleware.TokenRequired(), payment.Authorize)
	v2.POST("/order/:id/authorize", middleware.TokenRequired(), payment.Authorize)
	v2.POST("/order/:id/capture", middleware.TokenRequired(), payment.Capture)

	// Setup API routes
	logApiRoutes(entities)
	for _, entity := range entities {
		rest.New(entity).Route(router)
	}
}

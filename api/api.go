package api

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/api/payment"
	"crowdstart.io/middleware"
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
	router.GET("/access/:id", func(c *gin.Context) {
		id := c.Params.ByName("id")

		query := c.Request.URL.Query()
		email := query.Get("email")
		password := query.Get("password")

		getAccessToken(c, id, email, password)
	})

	router.POST("/access/:id", func(c *gin.Context) {
		id := c.Params.ByName("id")

		email := c.Request.Form.Get("email")
		password := c.Request.Form.Get("password")

		getAccessToken(c, id, email, password)
	})

	router.DELETE("/access", middleware.TokenRequired(), func(c *gin.Context) {
		deleteAccessToken(c)
	})

	// Authorization routes
	// One Step Payments
	router.POST("/charge", middleware.TokenRequired(), payment.Charge)
	router.POST("/order/:id/charge", middleware.TokenRequired(), payment.Charge)

	// Two Step Payments - "Auth & Capture"
	router.POST("/authorize", middleware.TokenRequired(), payment.Authorize)
	router.POST("/order/:id/authorize", middleware.TokenRequired(), payment.Authorize)
	router.POST("/order/:id/capture", middleware.TokenRequired(), payment.Capture)

	// Setup API routes
	logApiRoutes(entities)
	for _, entity := range entities {
		rest.New(entity).Route(router)
	}
}

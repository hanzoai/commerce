package api

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/api/accesstoken"
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
	"crowdstart.io/util/permission"
	"crowdstart.io/util/rest"
	"crowdstart.io/util/router"
)

func init() {
	router := router.New("api")

	router.GET("/v1/", func(c *gin.Context) {
		c.Data(410, "application/json", make([]byte, 0))
	})

	adminRequired := middleware.TokenRequired(permission.Admin)
	publishedRequired := middleware.TokenRequired(permission.Admin, permission.Published)

	// Access token API
	router.GET("/access/:id", accesstoken.Get)
	router.POST("/access/:id", accesstoken.Post)
	router.DELETE("/access/:id", adminRequired, accesstoken.Delete)

	// One Step Payment API
	router.POST("/charge", publishedRequired, payment.Charge)
	router.POST("/order/:id/charge", publishedRequired, payment.Charge)

	// Two Step Payment API ("Auth & Capture")
	router.POST("/authorize", publishedRequired, payment.Authorize)
	router.POST("/order/:id/authorize", publishedRequired, payment.Authorize)
	router.POST("/order/:id/capture", adminRequired, payment.Capture)

	// Entities with automatic RESTful API
	entities := []mixin.Entity{
		campaign.Campaign{},
		coupon.Coupon{},
		collection.Collection{},
		product.Product{},
		order.Order{},
		user.User{},
		variant.Variant{},
	}

	for _, entity := range entities {
		rest.New(entity).Route(router, adminRequired)
	}

	organizationApi := rest.New(organization.Organization{})
	organizationApi.DefaultNamespace = true
	organizationApi.Route(router, adminRequired)

	tokenApi := rest.New(token.Token{})
	tokenApi.DefaultNamespace = true
	tokenApi.Route(router, adminRequired)

	accountApi := rest.New(user.User{})
	accountApi.DefaultNamespace = true
	accountApi.Kind = "account"
	accountApi.Route(router, adminRequired)

	// REST API debugger
	router.GET("/", rest.DebugIndex(entities))
}

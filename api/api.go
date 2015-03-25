package api

import (
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

	adminRequired := middleware.TokenRequired(permission.Admin)
	publishedRequired := middleware.TokenRequired(permission.Admin, permission.Published)

	// Organization APIs, namespaced by organization

	// One Step Payment API
	router.POST("/charge", publishedRequired, payment.Charge)
	router.POST("/order/:id/charge", publishedRequired, payment.Charge)

	// Two Step Payment API ("Auth & Capture")
	router.POST("/authorize", publishedRequired, payment.Authorize)
	router.POST("/order/:id/authorize", publishedRequired, payment.Authorize)
	router.POST("/order/:id/capture", adminRequired, payment.Capture)

	// Entities with automatic RESTful API
	entities := []mixin.Entity{
		&coupon.Coupon{},
		&collection.Collection{},
		&product.Product{},
		&order.Order{},
		&user.User{},
		&variant.Variant{},
	}

	for _, entity := range entities {
		rest.New(entity).Route(router, adminRequired)
	}

	// Crowdstart APIs, using default namespace (internal use only)
	crowdstart := router.Group("/c/", adminRequired)

	campaign := rest.New(&campaign.Campaign{})
	campaign.DefaultNamespace = true
	campaign.Route(crowdstart)

	organization := rest.New(&organization.Organization{})
	organization.DefaultNamespace = true
	organization.Route(crowdstart)

	token := rest.New(&token.Token{})
	token.DefaultNamespace = true
	token.Route(crowdstart)

	user := rest.New(&user.User{})
	user.DefaultNamespace = true
	user.Route(crowdstart)

	// REST API debugger
	router.GET("/", rest.DebugIndex(entities))

	// Access token API (internal use only)
	router.GET("/access/:id", accesstoken.Get)
	router.POST("/access/:id", accesstoken.Post)
	router.DELETE("/access/:id", adminRequired, accesstoken.Delete)
}

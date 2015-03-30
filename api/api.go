package api

import (
	"crowdstart.io/api/accesstoken"
	"crowdstart.io/api/payment"
	"crowdstart.io/middleware"
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
	methodOverride := middleware.MethodOverride()

	// Organization APIs, namespaced by organization

	// One Step Payment API
	payment.Route(router)

	// Models with public RESTful API
	rest.New(coupon.Coupon{}).Route(router, adminRequired)
	rest.New(collection.Collection{}).Route(router, adminRequired)
	rest.New(product.Product{}).Route(router, adminRequired)
	rest.New(order.Order{}).Route(router, adminRequired)
	rest.New(user.User{}).Route(router, adminRequired)
	rest.New(variant.Variant{}).Route(router, adminRequired)

	// Crowdstart APIs, using default namespace (internal use only)
	campaign := rest.New(campaign.Campaign{})
	campaign.DefaultNamespace = true
	campaign.Prefix = "/c/"
	campaign.Route(router, adminRequired)

	organization := rest.New(organization.Organization{})
	organization.DefaultNamespace = true
	organization.Prefix = "/c/"
	organization.Route(router, adminRequired)

	token := rest.New(token.Token{})
	token.DefaultNamespace = true
	token.Prefix = "/c/"
	token.Route(router, adminRequired)

	user := rest.New(user.User{})
	user.DefaultNamespace = true
	user.Prefix = "/c/"
	user.Route(router, adminRequired)

	// Access token API (internal use only)
	accessToken := rest.New("/access")
	accessToken.GET("/:mode/:id", accesstoken.Get)
	accessToken.POST("/:mode/:id", adminRequired, accesstoken.Delete)
	accessToken.DELETE("/:mode/:id", adminRequired, accesstoken.Delete)
	accessToken.Route(router, methodOverride)

	// REST API debugger
	router.GET("/", middleware.ParseToken, rest.ListRoutes())
}

package api

import (
	"crowdstart.io/middleware"
	"crowdstart.io/models2/campaign"
	"crowdstart.io/models2/collection"
	"crowdstart.io/models2/coupon"
	"crowdstart.io/models2/organization"
	"crowdstart.io/models2/payment"
	"crowdstart.io/models2/product"
	"crowdstart.io/models2/token"
	"crowdstart.io/models2/user"
	"crowdstart.io/models2/variant"
	"crowdstart.io/util/permission"
	"crowdstart.io/util/rest"
	"crowdstart.io/util/router"

	accessTokenApi "crowdstart.io/api/accesstoken"
	orderApi "crowdstart.io/api/order"
	paymentApi "crowdstart.io/api/payment"
)

func init() {
	router := router.New("api")

	adminRequired := middleware.TokenRequired(permission.Admin)

	// Organization APIs, namespaced by organization

	// One Step Payment API
	paymentApi.Route(router)

	// Models with public RESTful API
	rest.New(coupon.Coupon{}).Route(router, adminRequired)
	rest.New(collection.Collection{}).Route(router, adminRequired)
	rest.New(product.Product{}).Route(router, adminRequired)
	rest.New(user.User{}).Route(router, adminRequired)
	rest.New(payment.Payment{}).Route(router, adminRequired)
	rest.New(variant.Variant{}).Route(router, adminRequired)
	orderApi.Route(router, adminRequired)

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
	accessTokenApi.Route(router)

	// REST API debugger
	router.GET("/", middleware.ParseToken, rest.ListRoutes())
}

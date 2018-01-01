package organization

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/api/organization/analytics"
	"hanzo.io/api/organization/integrations"
	"hanzo.io/api/organization/wallet"
	"hanzo.io/middleware"
	"hanzo.io/models/organization"
	"hanzo.io/util/permission"
	"hanzo.io/util/rest"
	"hanzo.io/util/router"
)

func Route(router router.Router, args ...gin.HandlerFunc) {
	adminRequired := middleware.TokenRequired(permission.Admin)
	namespaced := middleware.Namespace()

	api := rest.New(organization.Organization{})
	api.DefaultNamespace = true
	api.Prefix = "/c/"

	api.GET("/:organizationid/analytics", adminRequired, namespaced, analytics.Get)
	api.POST("/:organizationid/analytics", adminRequired, namespaced, analytics.Set)
	api.PUT("/:organizationid/analytics", adminRequired, namespaced, analytics.Set)
	api.PATCH("/:organizationid/analytics", adminRequired, namespaced, analytics.Update)

	api.GET("/:organizationid/integrations", adminRequired, namespaced, integrations.Get)
	api.POST("/:organizationid/integrations", adminRequired, namespaced, integrations.Upsert)
	api.PUT("/:organizationid/integrations", adminRequired, namespaced, integrations.Upsert)
	api.PATCH("/:organizationid/integrations", adminRequired, namespaced, integrations.Upsert)
	api.DELETE("/:organizationid/integrations/:integrationid", adminRequired, namespaced, integrations.Delete)

	api.GET("/:organizationid/wallet", adminRequired, namespaced, wallet.Get)
	api.GET("/:organizationid/wallet/account/:name", adminRequired, namespaced, wallet.GetAccount)
	api.POST("/:organizationid/wallet/account", adminRequired, namespaced, wallet.CreateAccount)
	api.POST("/:organizationid/wallet/send", adminRequired, namespaced, wallet.Send)

	api.Route(router, args...)
}

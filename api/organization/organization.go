package organization

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/api/organization/analytics"
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

	api.GET("/analytics", adminRequired, namespaced, analytics.Get)
	api.POST("/analytics", adminRequired, namespaced, analytics.Set)
	api.PUT("/analytics", adminRequired, namespaced, analytics.Set)
	api.PATCH("/analytics", adminRequired, namespaced, analytics.Update)

	api.Route(router, args...)
}

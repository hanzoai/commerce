package organization

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/api/organization/analytics"
	"crowdstart.com/middleware"
	"crowdstart.com/models/organization"
	"crowdstart.com/util/permission"
	"crowdstart.com/util/rest"
	"crowdstart.com/util/router"
)

func Route(router router.Router, args ...gin.HandlerFunc) {
	adminRequired := middleware.TokenRequired(permission.Admin)

	api := rest.New(organization.Organization{})
	api.DefaultNamespace = true
	api.Prefix = "/c/"

	api.GET("/:organizationid/analytics", adminRequired, analytics.Get)
	api.POST("/:organizationid/analytics", adminRequired, analytics.Set)
	api.PUT("/:organizationid/analytics", adminRequired, analytics.Set)
	api.PATCH("/:organizationid/analytics", adminRequired, analytics.Update)

	api.Route(router, args...)
}

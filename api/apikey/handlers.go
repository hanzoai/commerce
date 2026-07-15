package apikey

import (
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/apipermission"
	"github.com/hanzoai/commerce/models/publishableapikey"
	"github.com/hanzoai/commerce/models/role"
	"github.com/hanzoai/commerce/util/json/http"
	"github.com/hanzoai/commerce/util/rest"
)

func Route(router zip.Router, args ...zip.Handler) {
	namespaced := middleware.Namespace()

	api := rest.New(publishableapikey.PublishableApiKey{})
	api.POST("/:publishableapikeyid/revoke", namespaced, Revoke)
	api.Route(router, args...)

	// RBAC CRUD
	rest.New(role.Role{}).Route(router, args...)
	rest.New(apipermission.ApiPermission{}).Route(router, args...)
}

// Revoke marks an API key as revoked by setting RevokedAt to the current time.
func Revoke(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	id := c.Param("publishableapikeyid")

	k := publishableapikey.New(db)
	if err := k.GetById(id); err != nil {
		return http.Fail(c, 404, "No API key found with id: "+id, err)
	}

	now := time.Now()
	k.RevokedAt = &now

	// Set revokedBy from current user if available
	if u := c.Locals("user"); u != nil {
		k.RevokedBy = middleware.GetUser(c).Id()
	}

	if err := k.Update(); err != nil {
		return http.Fail(c, 500, "Failed to revoke API key", err)
	}

	return http.Render(c, 200, k)
}

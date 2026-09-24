package shipstation

import (
	"net/http"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/auth"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/thirdparty/shipstation/export"
	"github.com/hanzoai/commerce/thirdparty/shipstation/shipnotify"
)

func setOrg(c *zip.Ctx) error {
	db := datastore.New(c.Context())
	org := organization.New(db)
	if err := org.GetById(c.Param("organization")); err != nil {
		log.Panic("Organization not specified", c)
	}

	userid, err := auth.GetCurrentUserId(c)
	if err != nil {
		log.Panic("Unable to get current user", c)
	}

	if !(org.IsAdmin(userid) || org.IsOwner(userid)) {
		log.Panic("Not a valid admin/owner for this organization", c)
	}

	c.Locals("organization", org)
	return c.Next()
}

func Route(router *zip.Group, args ...zip.Handler) {
	api := router.Group("shipstation")

	basicAuth := middleware.BasicAuth()

	api.Raw(http.MethodGet, "/:organization", basicAuth, setOrg, export.Export)
	api.Raw(http.MethodPost, "/:organization", basicAuth, setOrg, shipnotify.ShipNotify)
}

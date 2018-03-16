package shipstation

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/auth"
	"hanzo.io/datastore"
	"hanzo.io/middleware"
	"hanzo.io/models/organization"
	"hanzo.io/thirdparty/shipstation/export"
	"hanzo.io/thirdparty/shipstation/shipnotify"
	"hanzo.io/log"
	"hanzo.io/util/router"
)

func setOrg(c *gin.Context) {
	db := datastore.New(c)
	org := organization.New(db)
	if err := org.GetById(c.Params.ByName("organization")); err != nil {
		log.Panic("Organization not specified", c)
	}

	userid, err := auth.GetCurrentUserId(c)
	if err != nil {
		log.Panic("Unable to get current user", c)
	}

	if !(org.IsAdmin(userid) || org.IsOwner(userid)) {
		log.Panic("Not a valid admin/owner for this organization", c)
	}

	c.Set("organization", org)
}

func Route(router router.Router, args ...gin.HandlerFunc) {
	api := router.Group("shipstation")

	basicAuth := middleware.BasicAuth()

	api.GET("/:organization", basicAuth, setOrg, export.Export)
	api.POST("/:organization", basicAuth, setOrg, shipnotify.ShipNotify)
}

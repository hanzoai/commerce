package shipstation

import (
	"log"

	"github.com/gin-gonic/gin"

	"crowdstart.com/auth"
	"crowdstart.com/datastore"
	"crowdstart.com/middleware"
	"crowdstart.com/models/organization"
	"crowdstart.com/thirdparty/shipstation/export"
	"crowdstart.com/thirdparty/shipstation/shipnotify"
	"crowdstart.com/util/router"
)

func setOrg(c *gin.Context) {
	db := datastore.New(c)
	org := organization.New(db)
	if err := org.GetById(c.Params.ByName("organization")); err != nil {
		log.Panic("Unable to get current user", c)
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

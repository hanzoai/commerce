package shipwire

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/datastore"
	"crowdstart.com/middleware"
	"crowdstart.com/models/organization"
	"crowdstart.com/thirdparty/shipwire/webhook"
	"crowdstart.com/util/log"
	"crowdstart.com/util/router"
)

func setOrg(c *gin.Context) {
	db := datastore.New(c)
	org := organization.New(db)
	if err := org.GetById(c.Params.ByName("organization")); err != nil {
		log.Panic("Organization not specified", c)
	}

	c.Set("organization", org)
}

func Route(router router.Router, args ...gin.HandlerFunc) {
	api := router.Group("shipstation")

	basicAuth := middleware.BasicAuth()

	api.POST("/:organization", basicAuth, webhook.Process)
}

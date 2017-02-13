package shipwire

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/models/organization"
	"hanzo.io/thirdparty/shipwire/webhook"
	"hanzo.io/util/log"
	"hanzo.io/util/router"
)

func setOrg(c *gin.Context) {
	db := datastore.New(c)
	org := organization.New(db)
	if err := org.GetById(c.Params.ByName("organization")); err != nil {
		log.Panic("Organization not specified", c)
	}

	c.Set("organization", org)
}

func Route(r router.Router, args ...gin.HandlerFunc) {
	api := r.Group("shipwire")

	api.HEAD("/:organization", setOrg, router.Ok)
	api.GET("/:organization", setOrg, webhook.Process)
	api.POST("/:organization", setOrg, webhook.Process)
}

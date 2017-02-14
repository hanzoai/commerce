package webhook

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/middleware"
	"hanzo.io/models/organization"
	"hanzo.io/util/log"
	"hanzo.io/util/permission"
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
	publishedRequired := middleware.TokenRequired(permission.Admin, permission.Published)

	api := r.Group("shipwire")
	api.HEAD("/webhook/:organization", setOrg, router.Ok)
	api.GET("/webhook/:organization", setOrg, webhook)
	api.POST("/webhook/:organization", setOrg, webhook)
	api.POST("/rate", publishedRequired, rate)
}

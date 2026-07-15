package api

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/util/permission"
	"github.com/hanzoai/commerce/util/router"
)

func setOrg(c *zip.Ctx) error {
	db := datastore.New(c.Context())
	org := organization.New(db)
	if err := org.GetById(c.Param("organization")); err != nil {
		log.Panic("Organization not specified", c)
	}

	c.Locals("organization", org)
	return c.Next()
}

func Route(r zip.Router, args ...zip.Handler) {
	adminRequired := middleware.TokenRequired(permission.Admin)
	publishedRequired := middleware.TokenRequired(permission.Admin, permission.Published)

	api := r.Group("shipwire")
	api.Head("/webhook/:organization", setOrg, router.Ok)
	api.Get("/webhook/:organization", setOrg, webhook)
	api.Post("/webhook/:organization", setOrg, webhook)

	api.Post("/return/:orderid", adminRequired, createReturn)
	api.Post("/order/:orderid", adminRequired, createOrder)
	api.Post("/rate", publishedRequired, rate)
}

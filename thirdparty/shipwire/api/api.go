package api

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/util/permission"
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
	// NO inbound webhook route. Shipwire ships no signing secret — there is no
	// Shipwire credential anywhere in this codebase — so the old
	// HEAD/GET/POST /shipwire/webhook/:organization derived the TENANT from a URL
	// path segment and mutated that org's orders, returns and tracking with an
	// unverified body. An external-counterparty callback is only ever trusted by
	// verifying ITS signature, and the tenant then follows from which credential
	// verified — never from the path. Restore this route together with that
	// verification, not before.

	api.Post("/return/:orderid", adminRequired, createReturn)
	api.Post("/order/:orderid", adminRequired, createOrder)
	api.Post("/rate", publishedRequired, rate)
}

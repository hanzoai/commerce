package zipctx

import (
	"context"
	"testing"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/organization"
)

// app backs the synthetic request contexts handed to handler-unit tests.
var app = zip.New(zip.Config{DisableStartupMessage: true})

// New returns a *zip.Ctx over a synthetic request, seeded with the default
// test locals — the handler-unit-test analog of a live request.
func New(ctx context.Context) *zip.Ctx {
	c := app.TestCtx("GET", "/")
	SetDefaults(c, ctx)
	return c
}

// SetDefaults seeds the locals every handler assumes a live middleware chain
// has set: the mint-gated request context, verbosity, live/test mode, and a
// fresh Organization bound to ctx's datastore.
func SetDefaults(c *zip.Ctx, ctx context.Context) {
	c.SetContext(ctx)
	c.Locals("verbose", testing.Verbose())
	c.Locals("test", false)
	db := datastore.New(ctx)
	c.Locals("organization", organization.New(db))
}

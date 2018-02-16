package gincontext

import (
	"log"
	"testing"

	"golang.org/x/net/context"

	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/models/organization"
)

func New(ctx ...context.Context) *context.Context {
	var _ctx context.Context

	switch len(ctx) {
	case 1:
		_ctx = ctx[0]
	default:
		log.Panic("At most one context.Context argument may be specified.")
	}

	// Setup default context.Context for tests
	c := new(context.Context)
	SetDefaults(c, _ctx)
	return c
}

func SetDefaults(c *context.Context, ctx context.Context) {
	c.Set("google.golang.org/appengine", ctx)
	c.Set("verbose", testing.Verbose())
	c.Set("test", false)
	db := datastore.New(ctx)
	c.Set("organization", organization.New(db))
}

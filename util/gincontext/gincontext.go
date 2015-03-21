package gincontext

import (
	"log"
	"testing"

	"github.com/gin-gonic/gin"

	"appengine"
)

func New(ctx ...appengine.Context) *gin.Context {
	var _ctx appengine.Context

	switch len(ctx) {
	case 1:
		_ctx = ctx[0]
	default:
		log.Panic("At most one appengine.Context argument may be specified.")
	}

	// Setup default gin Context for tests
	c := new(gin.Context)
	SetDefaults(c, _ctx)
	return c
}

func SetDefaults(c *gin.Context, ctx appengine.Context) {
	c.Set("appengine", ctx)
	c.Set("verbose", testing.Verbose())
	c.Set("test", false)
}

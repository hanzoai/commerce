package gin

import (
	"log"
	"testing"

	"crowdstart.io/util/test/ae"
	"github.com/gin-gonic/gin"

	"appengine"
)

func NewContext(ctx ...appengine.Context) *gin.Context {
	var _ctx appengine.Context

	switch len(ctx) {
	case 0:
		_ctx = ae.NewContext()
	case 1:
		_ctx = ctx[0]
	default:
		log.Panic("At most one appengine.Context argument may be specified.")
	}

	// Setup default gin Context for tests
	c := new(gin.Context)
	c.Set("appengine", _ctx)
	c.Set("verbose", testing.Verbose())
	c.Set("test", true)

	return c
}

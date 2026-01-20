package ae

import (
	ctx "context"
	"google.golang.org/appengine/aetest"

	"github.com/hanzoai/commerce/log"
)

var (
	SharedContext *context
	Counter       int
)

type Context interface {
	ctx.Context
	Close()
}

type context struct {
	ctx.Context
	instance aetest.Instance
}

func (c *context) Close() {
	Counter--

	if Counter == 0 {
		return
	}

	defer func() {
		if r := recover(); r != nil {
			log.Warn("Recovered from panic in instance.Close()")
		}
	}()
	if c.instance != nil {
		c.instance.Close()
	}
}

package ae

import (
	ctx "context"
	"google.golang.org/appengine/aetest"

	"hanzo.io/log"
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
	defer func() {
		if r := recover(); r != nil {
			log.Warn("Recovered from panic in instance.Close()")
		}
	}()
	c.instance.Close()
}

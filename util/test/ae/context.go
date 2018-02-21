package ae

import (
	ctx "golang.org/x/net/context"
)

type Context interface {
	ctx.Context
	Close()
}

type context struct {
	ctx.Context
}

func (c *context) Close() {
	Close()
}

package webhook

import (
	"context"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/models/webhook/tasks"
)

func Emit(ctx interface{}, org string, event string, data interface{}) {
	var reqCtx context.Context

	switch v := ctx.(type) {
	case *zip.Ctx:
		if c := v.Context(); c != nil {
			reqCtx = c.(context.Context)
		} else {
			reqCtx = v.Context()
		}
	case context.Context:
		reqCtx = v
	}

	// If we have a model, fire off a json-safe copy of it
	model, ok := data.(mixin.Entity)
	if ok {
		tasks.Emit.Call(reqCtx, org, event, model.CloneFromJSON())
	} else {
		tasks.Emit.Call(reqCtx, org, event, data)
	}
}

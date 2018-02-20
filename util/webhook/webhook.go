package webhook

import (
	"context"
	"github.com/gin-gonic/gin"

	"hanzo.io/models/mixin"
	"hanzo.io/models/webhook/tasks"
)

func Emit(ctx interface{}, org string, event string, data interface{}) {
	var aectx context.Context

	switch v := ctx.(type) {
	case *gin.Context:
		aectx = v.MustGet("appengine").(context.Context)
	case context.Context:
		aectx = v
	}

	// If we have a model, fire off a json-safe copy of it
	model, ok := data.(mixin.Entity)
	if ok {
		tasks.Emit.Call(aectx, org, event, model.CloneFromJSON())
	} else {
		tasks.Emit.Call(aectx, org, event, data)
	}
}

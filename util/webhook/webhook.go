package webhook

import (
	"github.com/gin-gonic/gin"

	"appengine"

	"crowdstart.com/models/mixin"
	"crowdstart.com/models/webhook/tasks"
)

func Emit(ctx interface{}, org string, event string, data interface{}) {
	var aectx appengine.Context

	switch v := ctx.(type) {
	case *gin.Context:
		aectx = v.MustGet("appengine").(appengine.Context)
	case appengine.Context:
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

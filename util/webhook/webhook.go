package webhook

import (
	"github.com/gin-gonic/gin"

	"appengine"

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

	tasks.Emit.Call(aectx, org, event, data)
}

package webhook

import (
	"github.com/gin-gonic/gin"

	"appengine"

	"crowdstart.com/models/webhook/tasks"
)

func Emit(ctx interface{}, org string, event string, data interface{}) {

	switch v := ctx.(type) {
	case *gin.Context:
		aectx := v.MustGet("appengine").(appengine.Context)
		tasks.Emit.Call(aectx, org, event, data)
	case appengine.Context:
		tasks.Emit.Call(v, org, event, data)
	}
}

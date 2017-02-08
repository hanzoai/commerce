package event

import (
	"github.com/gin-gonic/gin"

	"golang.org/x/net/context"

	"hanzo.io/models/mixin"
	"hanzo.io/util/json"
	"hanzo.io/util/log"
	"hanzo.io/util/webhook"
)

var Trigger func(context.Context, string, string, interface{}) error

func Emit(ctx interface{}, orgId string, event string, data interface{}) error {
	var (
		aectx context.Context
		err   error
	)

	switch v := ctx.(type) {
	case *gin.Context:
		aectx = v.MustGet("appengine").(context.Context)
	case context.Context:
		aectx = v
	}

	// Fire off corresponding webhooks

	// If we have a model, fire off a json-safe copy of it
	model, ok := data.(mixin.Entity)
	if ok {
		err = webhook.Emit(aectx, orgId, event, model.JSON())
	} else {
		err = webhook.Emit(aectx, orgId, event, json.EncodeBytes(data))
	}

	if err != nil {
		log.Error("Error while emitting %v: %v", event, err, aectx)
		return err
	}

	// Fire off triggers
	if Trigger != nil {
		err = Trigger(aectx, orgId, event, data)
	}

	if err != nil {
		log.Error("Error while triggering %v: %v", event, err, aectx)
		return err
	}

	return nil
}

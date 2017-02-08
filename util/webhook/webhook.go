package webhook

import (
	"golang.org/x/net/context"

	"hanzo.io/util/webhook/tasks"
)

func Emit(ctx context.Context, orgId string, event string, data []byte) error {
	// If we have a model, fire off a json-safe copy of it
	return tasks.Emit.Call(ctx, orgId, event, data)
}

package api

import (
	"hanzo.io/models/mixin"
	"hanzo.io/log"
)

func logApiRoutes(entities []mixin.Entity) {
	if len(entities) == 0 {
		return
	}

	message := "Registering API routes: " + entities[0].Kind()
	for _, entity := range entities[1:] {
		message += ", " + entity.Kind()
	}
	log.Info(message)
}

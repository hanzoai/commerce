package note

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/models/note"
	"hanzo.io/util/rest"
	"hanzo.io/util/router"
)

func Route(router router.Router, args ...gin.HandlerFunc) {
	api := rest.New(note.Note{})
	api.POST("/search", search)

	api.Route(router, args...)
}

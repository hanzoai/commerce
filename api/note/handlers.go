package note

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/middleware"
	"hanzo.io/models/note"
	"hanzo.io/util/permission"
	"hanzo.io/util/rest"
	"hanzo.io/util/router"
)

func Route(router router.Router, args ...gin.HandlerFunc) {
	adminRequired := middleware.TokenRequired(permission.Admin)
	origin := middleware.AccessControl("*")

	api := rest.New(note.Note{})
	api.Use(adminRequired, origin)
	api.POST("/search", search)

	api.Route(router, args...)
}

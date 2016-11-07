package http

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/middleware"
	"crowdstart.com/util/hashid"
	"crowdstart.com/util/router"
	"crowdstart.com/util/template"
)

// Setup handlers for HTTP registered tasks
func SetupRoutes(router router.Router) {
	// Redirects
	router.GET("/hashid", func(c *gin.Context) {
		template.Render(c, "hashid.html")
	})

	// Check a hashid
	router.POST("/hashid/:id", func(c *gin.Context) {
		ctx := middleware.GetAppEngine(c)
		id := c.Params.ByName("id")
		key, err := hashid.DecodeKey(ctx, id)
		if err != nil {
			panic(err)
		}
		template.Render(c, "hashid.html", "id", id, "kind", key.Kind(), "intind", key.IntID(), "namespace", key.Namespace())
	})
}

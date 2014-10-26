package store

import (
	"net/http"
	"github.com/gin-gonic/gin"
	"crowdstart.io/middleware"
	"crowdstart.io/templates"
)

func init() {
	router := gin.Default()

	router.Use(middleware.Host())
	router.Use(middleware.AppEngine())

	router.GET("/", func(c *gin.Context) {
		ctx := middleware.GetAppEngine(c)

		err := templates.Render(c, "products.html", nil); if err != nil {
			ctx.Errorf("%v", err)
			c.String(500, "Unable to render template")
		}
	})

	http.Handle("/", router)
}

package checkout

import (
	"net/http"
	"crowdstart.io/middleware"
	"github.com/gin-gonic/gin"
	"crowdstart.io/util/template"
)

func init() {
	router := gin.Default()

	router.Use(middleware.Host())
	router.Use(middleware.AppEngine())

	checkout := router.Group("/checkout")

	checkout.GET("/", func(c *gin.Context) {
		if err := template.Render(c, "checkout.html", nil); err != nil {
			c.String(500, "Unable to render template")
		}
	})

	http.Handle("/checkout/", router)
}

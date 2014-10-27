package checkout

import (
	"crowdstart.io/middleware"
	"crowdstart.io/util/template"
	"github.com/gin-gonic/gin"
	"net/http"
)

func init() {
	router := middleware.NewRouter()

	checkout := router.Group("/checkout")

	checkout.GET("/", func(c *gin.Context) {
		if err := template.Render(c, "checkout.html", nil); err != nil {
			c.String(500, "Unable to render template")
		}
	})

	http.Handle("/checkout/", router)
}

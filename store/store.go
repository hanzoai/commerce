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
		templates.Render(c, "store/product.html", nil)
	})

	http.Handle("/", router)
}

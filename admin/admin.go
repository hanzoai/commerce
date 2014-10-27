package admin

import (
	"net/http"
	"github.com/gin-gonic/gin"
	"crowdstart.io/util/router"
)

func init() {
	router := router.New()

	admin := router.Group("/admin")

	admin.GET("/", func(c *gin.Context) {
		c.String(200, "api")
	})

	http.Handle("/admin/", router)
}

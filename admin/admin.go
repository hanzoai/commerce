package admin

import (
	"net/http"
	"github.com/gin-gonic/gin"
	"crowdstart.io/middleware"
)

func init() {
	router := middleware.NewRouter()

	admin := router.Group("/admin")

	admin.GET("/", func(c *gin.Context) {
		c.String(200, "api")
	})

	http.Handle("/admin/", router)
}

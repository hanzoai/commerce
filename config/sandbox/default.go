package crowdstart

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/_default"
)

func init() {
	_default.Init()

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.GET("/", func(c *gin.Context) {
		c.Redirect(301, "http://www.crowdstart.com/docs")

	})
}

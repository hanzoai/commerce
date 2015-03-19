package crowdstart

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/_default"
)

func init() {
	_default.Init()

	router := gin.New()
	router.GET("/", func(c *gin.Context) {
		c.Redirect(301, "http://www.crowdstart.com/docs")

	})
}

package crowdstart

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/util/default_"
)

func init() {
	default_.Init()

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.GET("/", func(c *gin.Context) {
		c.Redirect(301, "http://hanzo.io/docs")

	})
}

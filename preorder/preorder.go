package preorder

import (
	"crowdstart.io/util/router"
	"crowdstart.io/util/template"
	"github.com/gin-gonic/gin"
)

func init() {
	router := router.New("preorder")

	router.GET("/", Index)
	router.POST("/", Login)

	router.GET("/order/:id", GetPreorder)
	router.POST("/order/save", SavePreorder)

	router.GET("/thanks", func(c *gin.Context) {
		template.Render(c, "thanks.html")
	})

	router.GET("/expired-token", func(c *gin.Context) {
		template.Render(c, "expired-token.html")
	})

}

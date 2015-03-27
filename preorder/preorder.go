package preorder

import (
	"crowdstart.io/middleware"
	"crowdstart.io/util/router"
	"crowdstart.io/util/template"
	"github.com/gin-gonic/gin"
)

func init() {
	router := router.New("preorder")

	loginRequired := middleware.LoginRequired("preorder")

	router.GET("/", Index)
	router.POST("/", Login)

	router.GET("/order/:id", loginRequired, GetPreorder)
	router.POST("/order/save", loginRequired, SavePreorder)

	router.GET("/thanks", func(c *gin.Context) {
		template.Render(c, "thanks.html")
	})

	router.GET("/expired-token", func(c *gin.Context) {
		template.Render(c, "expired-token.html")
	})

}

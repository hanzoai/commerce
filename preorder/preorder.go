package preorder

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/config"
	"crowdstart.io/middleware"
	"crowdstart.io/util/router"
	"crowdstart.io/util/template"
)

func init() {
	router := router.New("preorder")

	loginRequired := middleware.LoginRequired("preorder")

	router.GET("/", func(c *gin.Context) {
		c.Redirect(302, config.UrlFor("store"))
	})

	router.GET("/login", Login)
	router.POST("/login", LoginSubmit)

	router.GET("/order/:id", loginRequired, GetPreorder)
	router.POST("/order/save", loginRequired, SavePreorder)

	router.GET("/thanks", func(c *gin.Context) {
		template.Render(c, "thanks.html")
	})

	router.GET("/expired-token", func(c *gin.Context) {
		template.Render(c, "expired-token.html")
	})
}

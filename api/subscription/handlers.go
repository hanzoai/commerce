package subscription

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/config"
	"crowdstart.com/middleware"
	"crowdstart.com/util/json/http"
	"crowdstart.com/util/permission"
	"crowdstart.com/util/router"
)

var subscriptionEndpoint = config.UrlFor("api", "/subscription/")

func Subscribe(c *gin.Context) {
	org := middleware.GetOrganization(c)

	sub, _, err := subscribe(c, org)
	if err != nil {
		http.Fail(c, 500, "Error during subscribe", err)
		return
	}

	c.Writer.Header().Add("Location", subscriptionEndpoint+sub.Id())
	sub.Number = sub.NumberFromId()
	http.Render(c, 200, sub)
}

func Route(router router.Router, args ...gin.HandlerFunc) {
	api := router.Group("")
	api.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
	})

	publishedRequired := middleware.TokenRequired(permission.Admin, permission.Published)

	// Charge Payment API
	api.POST("/subscribe", publishedRequired, Subscribe)
}

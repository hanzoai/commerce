package accesstoken

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/middleware"
	"crowdstart.io/util/permission"
	"crowdstart.io/util/router"
)

// Access token routes
func Get(c *gin.Context) {
	id := c.Params.ByName("id")
	mode := c.Params.ByName("mode")
	test := false
	if mode == "test" {
		test = true
	}

	query := c.Request.URL.Query()
	email := query.Get("email")
	password := query.Get("password")

	getAccessToken(c, id, email, password, test)
}

func Post(c *gin.Context) {
	// If method override is used
	if c.Request.Method == "DELETE" {
		Delete(c)
		return
	}

	id := c.Params.ByName("id")
	mode := c.Params.ByName("mode")
	test := false
	if mode == "test" {
		test = true
	}

	email := c.Request.Form.Get("email")
	password := c.Request.Form.Get("password")
	getAccessToken(c, id, email, password, test)
}

func Delete(c *gin.Context) {
	deleteAccessToken(c)
}

func Route(router router.Router, args ...gin.HandlerFunc) {
	adminRequired := middleware.TokenRequired(permission.Admin)

	api := router.Group("/access")
	api.GET("/:mode/:id", Get)
	api.POST("/:mode/:id", adminRequired, Delete)
	api.DELETE("/:mode/:id", adminRequired, Delete)
}

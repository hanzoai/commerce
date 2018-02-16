package accesstoken

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/middleware"
	"hanzo.io/util/permission"
	"hanzo.io/util/router"
)

// Access token routes
func Get(c *context.Context) {
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

func Post(c *context.Context) {
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

func Delete(c *context.Context) {
	deleteAccessToken(c)
}

func Route(router router.Router, args ...gin.HandlerFunc) {
	adminRequired := middleware.TokenRequired(permission.Admin)

	api := router.Group("/access")
	api.GET("/:mode/:id", Get)
	api.POST("/:mode/:id", adminRequired, Delete)
	api.DELETE("/:mode/:id", adminRequired, Delete)
}

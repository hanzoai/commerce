package accesstoken

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/util/permission"
)

// Access token routes
func Get(c *zip.Ctx) error {
	id := c.Param("id")
	mode := c.Param("mode")
	test := false
	if mode == "test" {
		test = true
	}

	email := c.Query("email")
	password := c.Query("password")

	return getAccessToken(c, id, email, password, test)
}

func Post(c *zip.Ctx) error {
	// If method override is used
	if c.Method() == "DELETE" {
		return Delete(c)
	}

	id := c.Param("id")
	mode := c.Param("mode")
	test := false
	if mode == "test" {
		test = true
	}

	email := c.Fiber().FormValue("email")
	password := c.Fiber().FormValue("password")
	return getAccessToken(c, id, email, password, test)
}

func Delete(c *zip.Ctx) error {
	return deleteAccessToken(c)
}

func Route(router zip.Router, args ...zip.Handler) {
	adminRequired := middleware.TokenRequired(permission.Admin)

	api := router.Group("/access")
	api.Get("/:mode/:id", Get)
	api.Post("/:mode/:id", adminRequired, Delete)
	api.Delete("/:mode/:id", adminRequired, Delete)
}

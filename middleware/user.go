package middleware

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/auth"
	"github.com/hanzoai/commerce/config"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/user"
	"github.com/hanzoai/commerce/util/session"
)

func AcquireUser(moduleName string) zip.Handler {
	return func(c *zip.Ctx) error {
		u, err := auth.GetCurrentUser(c)
		if err != nil {
			log.Warn("Unable to acquire user.", c)
			session.Clear(c)
			return c.Redirect(302, config.UrlFor(moduleName, "/login"))
		}
		c.Locals("user", u)
		return c.Next()
	}
}

func GetCurrentUser(c *zip.Ctx) *user.User {
	return c.Locals("user").(*user.User)
}

package middleware

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/auth"
	"github.com/hanzoai/commerce/config"
	"github.com/hanzoai/commerce/log"
)

// Updates session with login information, does not require it
func CheckLogin() zip.Handler {
	return func(c *zip.Ctx) error {
		if auth.IsLoggedIn(c) {
			u, err := auth.GetCurrentUser(c)
			if err != nil {
				return c.Next()
			}
			auth.Login(c, u)
		}
		return c.Next()
	}
}

// Require login to view route
func LoginRequired(moduleName string) zip.Handler {
	return func(c *zip.Ctx) error {
		if auth.IsLoggedIn(c) {
			return c.Next()
		}

		log.Warn("Access denied, redirecting to login page", c)
		return c.Redirect(302, config.UrlFor(moduleName, "/login"))
	}
}

// Required to be logged out to view
func LogoutRequired(moduleName string) zip.Handler {
	return func(c *zip.Ctx) error {
		if !auth.IsLoggedIn(c) {
			return c.Next()
		}

		log.Warn("Already logged in, redirecting to module", c)
		return c.Redirect(302, config.UrlFor(moduleName))
	}
}

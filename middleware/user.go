package middleware

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/auth"
	"hanzo.io/config"
	"hanzo.io/models/user"
	"hanzo.io/log"
	"hanzo.io/util/session"
)

func AcquireUser(moduleName string) gin.HandlerFunc {
	return func(c *context.Context) {
		if u, err := auth.GetCurrentUser(c); err != nil {
			log.Warn("Unable to acquire user.", c)
			session.Clear(c)
			c.Redirect(302, config.UrlFor(moduleName, "/login"))
			c.AbortWithStatus(302)
		} else {
			c.Set("user", u)
		}
	}
}

func GetCurrentUser(c *context.Context) *user.User {
	return c.MustGet("user").(*user.User)
}

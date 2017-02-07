package middleware

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/auth"
	"crowdstart.com/config"
	"crowdstart.com/models/user"
	"crowdstart.com/util/log"
	"crowdstart.com/util/session"
)

func AcquireUser(moduleName string) gin.HandlerFunc {
	return func(c *gin.Context) {
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

func GetCurrentUser(c *gin.Context) *user.User {
	return c.MustGet("user").(*user.User)
}

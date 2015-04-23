package middleware

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/auth"
	"crowdstart.io/config"
	"crowdstart.io/models2/user"
	"crowdstart.io/util/log"
	"crowdstart.io/util/session"
)

func AcquireUser(moduleName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if u, err := auth.GetCurrentUser(c); err != nil {
			log.Warn("Unable to acquire user.")
			session.Clear(c)
			c.Redirect(302, config.UrlFor(moduleName, "/login"))
			c.Abort(302)
		} else {
			c.Set("user", u)
		}
	}
}

func GetCurrentUser(c *gin.Context) *user.User {
	return c.MustGet("user").(*user.User)
}

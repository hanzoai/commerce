package middleware

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/auth2"
	"crowdstart.io/config"
	"crowdstart.io/util/log"
)

// Updates session with login information, does not require it
func CheckLogin() gin.HandlerFunc {
	return func(c *gin.Context) {
		loggedIn := auth.IsLoggedIn(c)
		if loggedIn {
			u, err := auth.GetCurrentUser(c)
			if err != nil {
				return
			}
			auth.Login(c, u)
		}
	}
}

// Require login to view route
func LoginRequired(moduleName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if auth.IsLoggedIn(c) {
			return
		}

		log.Warn("Access denied, redirecting to login page")
		c.Redirect(302, config.UrlFor(moduleName, "/login"))
		c.Abort(302)
	}
}

// Required to be logged out to view
func LogoutRequired(moduleName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !auth.IsLoggedIn(c) {
			return
		}

		log.Warn("Already logged in, redirecting to profile")
		c.Redirect(302, config.UrlFor(moduleName, "/profile"))
		c.Abort(302)
	}
}

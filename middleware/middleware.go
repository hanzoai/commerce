package middleware

import (
	"appengine"
	"crowdstart.io/auth"
	"github.com/gin-gonic/gin"
)

// Automatically get App Engine context.
func AppEngine() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := appengine.NewContext(c.Request)
		c.Set("appengine", ctx)
	}
}

// Automatically get the Host header so we can decide what to do with a given
// request.
func AddHost() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("host", c.Request.Header.Get("Host"))
	}
}

// Login Required middleware
func LoginRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !auth.IsLoggedIn(c) {
			c.Redirect(301, "/user/login")
		}
	}
}

func LoggedOutRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		if auth.IsLoggedIn(c) {
			c.Redirect(301, "/")
		}
	}
}

func GetAppEngine(c *gin.Context) appengine.Context {
	return c.MustGet("appengine").(appengine.Context)
}

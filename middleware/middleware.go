package middleware

import (
	"appengine"

	"github.com/gin-gonic/gin"

	"crowdstart.io/auth"
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

func LiveReload() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		// If 200 and text/html content-type inject bebop script for live reload
		if c.Writer.Status() == 200 && c.Writer.Header().Get("Content-Type") == "text/html" {
			c.Writer.Write([]byte("<!-- Live reload via bebop -->"))
			c.Writer.Write([]byte("<script src=\"http://localhost:1987/bebop-client/bebop.js\"></script>"))
			c.Writer.Write([]byte("<script>(new Bebop({port: 1987})).connect()</script>"))
		}
	}
}

func GetAppEngine(c *gin.Context) appengine.Context {
	return c.MustGet("appengine").(appengine.Context)
}

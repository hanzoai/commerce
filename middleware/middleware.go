package middleware

import (
	"appengine"

	"github.com/gin-gonic/gin"

	"crowdstart.io/auth"
	"crowdstart.io/util/log"
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
		c.Writer.WriteHeaderNow()
		contentType := c.Writer.Header().Get("Content-Type")
		status := c.Writer.Status()
		log.Debug("url: %v, status: %d, content-type: %s", c.Request.URL, status, contentType)
		if status == 200 && contentType == "text/html" {
			c.Writer.Write([]byte("<script src=\"http://localhost:3000/_bebop/\"></script>"))
		}
	}
}

func GetAppEngine(c *gin.Context) appengine.Context {
	return c.MustGet("appengine").(appengine.Context)
}

package middleware

import (
	"google.golang.org/appengine"

	"github.com/gin-gonic/gin"
)

// Automatically get App Engine context.
func AppEngine() gin.HandlerFunc {
	return func(c *context.Context) {
		ctx := appengine.NewContext(c.Request)
		c.Set("appengine", ctx)
	}
}

// Automatically get the Host header so we can decide what to do with a given
// request.
func AddHost() gin.HandlerFunc {
	return func(c *context.Context) {
		c.Set("host", c.Request.Header.Get("Host"))
	}
}

func LiveReload() gin.HandlerFunc {
	return func(c *context.Context) {
		c.Next()
		// If 200 and text/html content-type inject bebop script for live reload
		if c.Writer.Status() == 200 && c.Writer.Header().Get("Content-Type") == "text/html; charset=utf-8" {
			injectScript := []byte(`<!-- Live reload via bebop -->
<script src="http://localhost:8090/bebop.min.js"></script>
<script>try {(new Bebop({port: 8090})).connect()} catch(err) {}</script>`)
			c.Writer.Write(injectScript)
		}
	}
}

func GetAppEngine(c *context.Context) context.Context {
	return c.MustGet("appengine").(context.Context)
}

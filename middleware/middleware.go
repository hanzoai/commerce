package middleware

import (
	"appengine"

	"github.com/gin-gonic/gin"

	"crowdstart.io/auth"
	"crowdstart.io/config"
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

// Updates session with login information, does not require it
func CheckLogin() gin.HandlerFunc {
	return func(c *gin.Context) {
		loggedIn := auth.IsLoggedIn(c)
		log.Debug("loggedIn: %v", loggedIn)
		c.Set("logged-in", loggedIn)
	}
}

// Require login to view route
func LoginRequired(moduleName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		v, err := c.Get("logged-in")
		loggedIn, _ := v.(bool)

		if err != nil {
			loggedIn = auth.IsLoggedIn(c)
			c.Set("logged-in", loggedIn)
		}

		if !loggedIn {
			log.Debug("Redirecting to login page")
			c.Redirect(302, config.UrlFor(moduleName, "/login"))
			c.Abort(302)
		}
	}
}

// Required to be logged out to view
func LogoutRequired(moduleName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		v, err := c.Get("logged-in")
		loggedIn, _ := v.(bool)

		if err != nil {
			loggedIn = auth.IsLoggedIn(c)
			c.Set("logged-in", loggedIn)
		}

		if loggedIn {
			c.Redirect(302, config.UrlFor(moduleName, "/profile"))
		}
	}
}

func LiveReload() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		// If 200 and text/html content-type inject bebop script for live reload
		if c.Writer.Status() == 200 && c.Writer.Header().Get("Content-Type") == "text/html; charset=utf-8" {
			injectScript := []byte(`<!-- Live reload via bebop -->
<script src="http://localhost:1987/bebop-client/bebop.js"></script>
<script>try {(new Bebop({port: 1987})).connect()} catch(err) {}</script>`)
			c.Writer.Write(injectScript)
		}
	}
}

func GetAppEngine(c *gin.Context) appengine.Context {
	return c.MustGet("appengine").(appengine.Context)
}

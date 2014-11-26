package _default

import (
	"appengine"
	"crowdstart.io/config"
	"crowdstart.io/models/fixtures"
	"crowdstart.io/util/exec"
	"crowdstart.io/util/router"
	"github.com/gin-gonic/gin"
)

func Init() {
	router := router.New("default")

	router.GET("/", func(c *gin.Context) {
		if config.IsDevelopment {
			c.Writer.Header().Set("Content-Type", "text/html; charset=utf-8")
			c.Writer.WriteHeader(200)
			c.Writer.Write([]byte(`
			<html>
				<head>
					<title>crowdstart</title>
					<meta http-equiv="Content-Type" content="text/html; charset=utf-8" />
					<style>
						body {
							font-family:monospace;
							margin:20px
						}

						ul {

						}
					</style>
				</head>
				<body>
					<h4>200 ok (crowdstart/1.0.0)</h4>

					<ul>
						<li><a href="/api/">api</a></li>
						<li><a href="/checkout/">checkout</a></li>
						<li><a href="/platform/">platform</a></li>
						<li><a href="/preorder/">preorder</a></li>
						<li><a href="/store/">store</a></li>
					</ul>

					<a href="http://localhost:8000">admin</a>
				</body>
			</html>`))
		} else {
			c.Redirect(301, "http://www.crowdstart.io")
		}
	})

	// Warmup: install fixtures, etc.
	// Only used in development
	router.GET("/_ah/warmup", func(c *gin.Context) {
		if config.IsProduction {
			c.String(200, "Not utilized in production")
			return
		}

		// Automatically load fixtures
		if config.AutoLoadFixtures {
			ctx := appengine.NewContext(c.Request)
			fixtures.All.Call(ctx)
		}

		// Recompile static assets
		if config.AutoCompileAssets {
			exec.Run("make assets")
		}
	})

	router.GET("/fixtures/all", func(c *gin.Context) {
		ctx := appengine.NewContext(c.Request)

		// Start install-fixtures task
		fixtures.All.Call(ctx)

		c.String(200, "Fixtures installing...")
	})

	router.GET("/fixtures/international", func(c *gin.Context) {
		ctx := appengine.NewContext(c.Request)

		// Start install-fixtures task
		fixtures.International.Call(ctx)

		c.String(200, "Fixtures installing...")
	})
}

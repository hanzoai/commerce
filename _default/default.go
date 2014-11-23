package _default

import (
	"appengine"
	"crowdstart.io/config"
	"crowdstart.io/datastore"
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
						<li><a href="/api">api</a></li>
						<li><a href="/checkout">checkout</a></li>
						<li><a href="/platform">platform</a></li>
						<li><a href="/preorder">preorder</a></li>
						<li><a href="/store">store</a></li>
					</ul>

					<a href="http://localhost:8000">admin</a>
				</body>
			</html>`))
		} else {
			c.Redirect(301, "http://www.crowdstart.io")
		}
	})

	// Warmup: install fixtures, etc.
	router.GET("/_ah/warmup", func(c *gin.Context) {
		ctx := appengine.NewContext(c.Request)
		db := datastore.New(ctx)
		fixtures.Install(db)

		conf := config.Get()

		// Recompile static assets
		if conf.AutoCompileAssets {
			exec.Run("make autocompile-assets")
		}
	})
}

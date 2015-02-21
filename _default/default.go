package _default

import (
	_ "appengine/remote_api"

	"github.com/gin-gonic/gin"

	"crowdstart.io/config"
	"crowdstart.io/middleware"
	"crowdstart.io/util/exec"
	"crowdstart.io/util/router"
	"crowdstart.io/util/tasks"

	// Imported for side-effect of having tasks registered.
	_ "crowdstart.io/models/fixtures/tasks"
	_ "crowdstart.io/models/migrations/tasks"
	_ "crowdstart.io/test/datastore/integration/tasks"
	_ "crowdstart.io/thirdparty/mandrill/tasks"
)

func Init() {
	router := router.New("default")

	// Handler for HTTP registered tasks
	router.GET("/task/:name", func(c *gin.Context) {
		name := c.Params.ByName("name")
		tasks.Run(c, name)
		c.String(200, "Running task "+name)
	})

	if config.IsProduction {
		return
	}

	// Development routes

	// Static assets
	router.GET("/static/*file", middleware.Static("static/"))
	router.GET("/assets/*file", middleware.Static("assets/"))

	// Development index links to modules
	router.GET("/", func(c *gin.Context) {
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
	})

	// Warmup: automatically install fixtures, etc.
	router.GET("/_ah/warmup", func(c *gin.Context) {
		// Automatically load fixtures
		if config.AutoLoadFixtures {
			tasks.Run(c, "fixtures-install-all")
		}

		// Recompile static assets
		if config.AutoCompileAssets {
			exec.Run("make assets")
		}
	})
}

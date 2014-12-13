package _default

import (
	"net/http"

	"appengine"
	_ "appengine/remote_api"

	"github.com/gin-gonic/gin"

	"crowdstart.io/config"
	"crowdstart.io/models/fixtures"
	"crowdstart.io/models/migrations"
	_ "crowdstart.io/thirdparty/mandrill"
	"crowdstart.io/util/exec"
	"crowdstart.io/util/router"
)

func Init() {
	router := router.New("default")

	router.GET("/fixtures/:fixture", func(c *gin.Context) {
		fixture := c.Params.ByName("fixture")
		ctx := appengine.NewContext(c.Request)

		// Call fixture task
		fixtures.Install.Call(ctx, fixture)

		c.String(200, "Fixtures installing...")
	})

	router.GET("/migrations/:migration", func(c *gin.Context) {
		migration := c.Params.ByName("migration")
		ctx := appengine.NewContext(c.Request)

		// Call fixture task
		migrations.Run.Call(ctx, migration)

		c.String(200, "Running migration...")
	})

	if config.IsProduction {
		return
	}

	// Development routes
	fs := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

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
			ctx := appengine.NewContext(c.Request)
			fixtures.Install.Call(ctx, "all")
		}

		// Recompile static assets
		if config.AutoCompileAssets {
			exec.Run("make assets")
		}
	})

}

package _default

import (
	"appengine"
	"appengine/datastore"
	_ "appengine/remote_api"

	"github.com/gin-gonic/gin"

	"crowdstart.io/config"
	"crowdstart.io/middleware"
	"crowdstart.io/models/fixtures"
	"crowdstart.io/models/migrations"
	"crowdstart.io/thirdparty/salesforce"
	"crowdstart.io/util/exec"
	"crowdstart.io/util/router"

	// Imported for side-effect of having tasks registered.
	_ "crowdstart.io/thirdparty/mandrill"
	_ "crowdstart.io/thirdparty/salesforce"

	// Only used in tests
	_ "crowdstart.io/test/datastore/parallel/worker"
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

	router.GET("/jobs/sync-salesforce", func(c *gin.Context) {
		ctx := appengine.NewContext(c.Request)

		// Check status of salesforce import
		m := migrations.MigrationStatus{}
		mk := datastore.NewKey(ctx, "migration", "SalesforceImportUsersTask", 0, nil)
		if err := datastore.Get(ctx, mk, &m); err != nil {
			// If we get an error, then no migration has been done and it is okay to sync
			salesforce.CallPullUpdatedTask(ctx)
		} else if m.Done {
			// If migration is done, then sync
			salesforce.CallPullUpdatedTask(ctx)
		}

		c.String(200, "Running job...")
	})

	router.GET("/jobs/import-users-to-salesforce", func(c *gin.Context) {
		ctx := appengine.NewContext(c.Request)
		salesforce.ImportUsers(ctx)

		c.String(200, "Running job...")
	})

	router.GET("/jobs/import-orders-to-salesforce", func(c *gin.Context) {
		ctx := appengine.NewContext(c.Request)
		salesforce.ImportOrders(ctx)

		c.String(200, "Running job...")
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
			ctx := appengine.NewContext(c.Request)
			fixtures.Install.Call(ctx, "all")
		}

		// Recompile static assets
		if config.AutoCompileAssets {
			exec.Run("make assets")
		}
	})
}

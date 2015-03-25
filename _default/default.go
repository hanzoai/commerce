package _default

import (
	"appengine"

	"github.com/gin-gonic/gin"

	"crowdstart.io/config"
	"crowdstart.io/middleware"
	"crowdstart.io/util/exec"
	"crowdstart.io/util/log"
	"crowdstart.io/util/router"
	"crowdstart.io/util/task"
	"crowdstart.io/util/template"

	// Imported for side-effect, needed to enable remote api calls
	_ "appengine/remote_api"

	// Imported for side-effect, ensures tasks are registered
	_ "crowdstart.io/models/fixtures"
	_ "crowdstart.io/models/migrations"
	_ "crowdstart.io/models2/fixtures"
	_ "crowdstart.io/thirdparty/mandrill/tasks"
	_ "crowdstart.io/thirdparty/salesforce/tasks"
	_ "crowdstart.io/thirdparty/stripe/tasks"
)

func Init() {
	router := router.New("default")

	// Setup routes for tasks
	task.SetupRoutes(router)

	// Index
	router.GET("/", func(c *gin.Context) {
		if appengine.IsDevAppServer() {
			// Development index links to modules
			template.Render(c, "index.html")
		} else {
			c.Data(500, "text/html", make([]byte, 0))
		}
	})

	// Monitoring test
	router.GET("/wake-up", func(c *gin.Context) {
		log.Panic("I think I heard, I think I heard a shot.")
	})

	// Development-only routes below
	if config.IsProduction {
		return
	}

	// Static assets
	router.GET("/static/*file", middleware.Static("static/"))
	router.GET("/assets/*file", middleware.Static("assets/"))

	// Warmup: automatically install fixtures, etc.
	router.GET("/_ah/warmup", func(c *gin.Context) {
		// Automatically load fixtures
		if config.AutoLoadFixtures {
			task.Run(c, "fixtures-all")
		}

		// Recompile static assets
		if config.AutoCompileAssets {
			exec.Run("make assets")
		}
	})
}

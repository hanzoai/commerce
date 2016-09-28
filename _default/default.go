package _default

import (
	"appengine"

	"github.com/gin-gonic/gin"

	"crowdstart.com/config"
	"crowdstart.com/middleware"
	"crowdstart.com/util/exec"
	"crowdstart.com/util/log"
	"crowdstart.com/util/router"
	"crowdstart.com/util/task"
	"crowdstart.com/util/template"

	// Imported for side-effect, needed to enable remote api calls
	_ "appengine/remote_api"

	// Imported for side-effect, ensures tasks are registered
	_ "crowdstart.com/cron/tasks"
	_ "crowdstart.com/models/fixtures"
	_ "crowdstart.com/models/migrations"
	_ "crowdstart.com/models/webhook/tasks"
	_ "crowdstart.com/thirdparty/mailchimp/tasks"
	_ "crowdstart.com/thirdparty/mandrill/tasks"
	_ "crowdstart.com/util/aggregate/tasks"
	_ "crowdstart.com/util/analytics/tasks"
	// _ "crowdstart.com/thirdparty/salesforce/tasks"
	_ "crowdstart.com/thirdparty/stripe/tasks"
)

func Init() {
	gin.SetMode(gin.ReleaseMode)

	router := router.New("default")

	// Index, development has nice index with links
	if appengine.IsDevAppServer() {
		router.GET("/", func(c *gin.Context) {
			template.Render(c, "index.html")
		})
	} else {
		router.GET("/", func(c *gin.Context) {
			c.String(200, "ok")
		})
	}

	// Monitoring test
	router.GET("/wake-up", func(c *gin.Context) {
		log.Panic("I think I heard, I think I heard a shot.")
	})

	// Setup routes for tasks
	task.SetupRoutes(router)

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

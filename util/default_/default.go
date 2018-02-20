package default_

import (
	"google.golang.org/appengine"

	"github.com/gin-gonic/gin"

	"hanzo.io/config"
	"hanzo.io/delay"
	"hanzo.io/log"
	"hanzo.io/middleware"
	"hanzo.io/util/exec"
	hashid "hanzo.io/util/hashid/http"
	"hanzo.io/util/router"
	"hanzo.io/util/task"
	"hanzo.io/util/template"

	// Imported for side-effect, needed to enable remote api calls
	_ "google.golang.org/appengine/remote_api"

	// Imported for side-effect, ensures tasks are registered
	_ "hanzo.io/api/checkout/tasks"
	_ "hanzo.io/cron/tasks"
	_ "hanzo.io/models/analyticsidentifier/tasks"
	_ "hanzo.io/models/fixtures"
	_ "hanzo.io/models/fixtures/users"
	_ "hanzo.io/models/migrations"
	_ "hanzo.io/models/referrer/tasks"
	_ "hanzo.io/models/webhook/tasks"
	_ "hanzo.io/thirdparty/mailchimp/tasks"
	_ "hanzo.io/thirdparty/mandrill/tasks"
	_ "hanzo.io/util/aggregate/tasks"
	_ "hanzo.io/util/analytics/tasks"
	// _ "hanzo.io/thirdparty/salesforce/tasks"
	_ "hanzo.io/thirdparty/stripe/tasks"
)

func Init() {
	gin.SetMode(gin.ReleaseMode)

	router := router.New("default")

	// Setup routes for delay funcs
	router.POST(delay.Path, func(c *gin.Context) {
		ctx := appengine.NewContext(c.Request)
		delay.RunFunc(ctx, c.Writer, c.Request)
	})

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

	// Setup hashid routes
	hashid.SetupRoutes(router)

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

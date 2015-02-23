package _default

import (
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
	_ "crowdstart.io/thirdparty/mandrill/tasks"
	_ "crowdstart.io/thirdparty/salesforce/tasks"
)

var Foo = task.Func("foo", func(c *gin.Context) {
	log.Debug("FOOOOOOOO")
})

func Init() {
	router := router.New("default")

	// Handler for HTTP registered tasks
	router.GET("/tasks", func(c *gin.Context) {
		template.Render(c, "tasks.html", "tasks", task.Names())
	})

	router.GET("/task/", func(c *gin.Context) {
		c.Redirect(301, "/tasks")
	})

	router.GET("/task/:name", func(c *gin.Context) {
		name := c.Params.ByName("name")
		task.Run(c, name)
		template.Render(c, "task-running.html", "task", name)
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
		template.Render(c, "index.html")
	})

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

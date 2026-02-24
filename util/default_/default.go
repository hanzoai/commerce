package default_

import (
	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/config"
	"github.com/hanzoai/commerce/delay"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	// "github.com/hanzoai/commerce/util/exec"
	hashid "github.com/hanzoai/commerce/util/hashid/http"
	"github.com/hanzoai/commerce/util/router"
	"github.com/hanzoai/commerce/util/task"
	"github.com/hanzoai/commerce/util/template"

	// Imported for side-effect, ensures tasks are registered
	_ "github.com/hanzoai/commerce/api/checkout/tasks"
	_ "github.com/hanzoai/commerce/cron/tasks"
	_ "github.com/hanzoai/commerce/email/tasks"
	_ "github.com/hanzoai/commerce/models/fixtures"
	_ "github.com/hanzoai/commerce/models/fixtures/users"
	_ "github.com/hanzoai/commerce/models/migrations"
	_ "github.com/hanzoai/commerce/models/referrer/tasks"
	_ "github.com/hanzoai/commerce/models/webhook/tasks"
	_ "github.com/hanzoai/commerce/util/aggregate/tasks"
	// _ "github.com/hanzoai/commerce/thirdparty/salesforce/tasks"
	_ "github.com/hanzoai/commerce/thirdparty/stripe/tasks"
)

func Init() {
	gin.SetMode(gin.ReleaseMode)

	router := router.New("default")

	// Index, development has nice index with links
	if config.IsDevelopment {
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

	// Setup routes for delay funcs
	router.POST(delay.Path, func(c *gin.Context) {
		ctx := middleware.GetContext(c)
		delay.RunFunc(ctx, c.Writer, c.Request)
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
}

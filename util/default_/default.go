package default_

import (
	"net/http"

	"github.com/zap-proto/fiber/v3/middleware/adaptor"
	"github.com/zap-proto/zip"

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
)

func Init(app *zip.App) {
	router := router.New(app, "default")

	// Index, development has nice index with links
	if config.IsDevelopment {
		router.Get("/", func(c *zip.Ctx) error {
			return template.Render(c, "index.html")
		})
	} else {
		router.Get("/", func(c *zip.Ctx) error {
			return c.String(200, "ok")
		})
	}

	// Monitoring test
	router.Get("/wake-up", func(c *zip.Ctx) error {
		log.Panic("I think I heard, I think I heard a shot.")
		return nil
	})

	// Setup routes for delay funcs. delay.RunFunc is net/http-native, so bridge
	// it through the fiber adaptor.
	router.Post(delay.Path, func(c *zip.Ctx) error {
		ctx := c.Context()
		return adaptor.HTTPHandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			delay.RunFunc(ctx, w, req)
		})(c.Fiber())
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
	router.Get("/static/*file", middleware.Static("static/"))
	router.Get("/assets/*file", middleware.Static("assets/"))
}

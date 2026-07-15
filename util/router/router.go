package router

import (
	"strings"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/config"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
)

// New mounts the module's route group on app under its configured prefix, with
// the core middleware chain applied. It returns the zip.Router the module
// registers its routes on. The /_ah warmup/start/stop probes are registered on
// the app root (never under the module prefix).
func New(app *zip.App, moduleName string) zip.Router {
	prefix := strings.TrimSpace(config.Prefixes[moduleName])
	if prefix == "" {
		log.Panic("Unable to determine prefix for module: '%s'", moduleName)
	}

	log.Info("Routing %s to %s", prefix, moduleName)

	group := app.Group(prefix)

	group.Use(middleware.Logger())

	// Special error handler for API module returns JSON always
	if moduleName == "api" {
		group.Use(middleware.ErrorHandlerJSON())
	} else {
		group.Use(middleware.ErrorHandler())
	}

	group.Use(middleware.NotFoundHandler())
	group.Use(middleware.AddHost())
	group.Use(middleware.RequestContext())
	group.Use(middleware.DetectOverrides())

	app.Get("/_ah/warmup", Ok)
	app.Get("/_ah/start", Ok)
	app.Get("/_ah/stop", Ok)

	return group
}

func Ok(c *zip.Ctx) error {
	return c.String(200, "ok\n")
}

func Empty(c *zip.Ctx) error {
	return c.NoContent(200)
}

func Robots(c *zip.Ctx) error {
	return c.String(200, "User-agent: *\nDisallow: /\n")
}

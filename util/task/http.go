package task

import (
	nethttp "net/http"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/util/json/http"
	"github.com/hanzoai/commerce/util/template"
)

// Setup handlers for HTTP registered tasks
func SetupRoutes(router *zip.Group) {
	// Redirects
	router.Raw(nethttp.MethodGet, "/task", func(c *zip.Ctx) error {
		return c.Redirect(301, "/tasks")
	})

	router.Raw(nethttp.MethodGet, "/tasks", func(c *zip.Ctx) error {
		return template.Render(c, "tasks.html", "tasks", Names())
	})

	// Show task
	router.Raw(nethttp.MethodGet, "/task/:name", func(c *zip.Ctx) error {
		name := c.Param("name")
		return template.Render(c, "task.html", "task", name)
	})

	// Run task
	router.Raw(nethttp.MethodPost, "/task/:name", func(c *zip.Ctx) error {
		name := c.Param("name")
		Run(c, name)
		return template.Render(c, "task-running.html", "task", name)
	})

	router.Raw(nethttp.MethodGet, "/run-tasks", func(c *zip.Ctx) error {
		return http.Render(c, 200, Names())
	})

	router.Raw(nethttp.MethodGet, "/run-task/:name", func(c *zip.Ctx) error {
		name := c.Param("name")
		Run(c, name)
		return c.Redirect(301, "/run-task/"+name+"/started")
	})

	router.Raw(nethttp.MethodGet, "/run-task/:name/started", func(c *zip.Ctx) error {
		name := c.Param("name")
		return http.Render(c, 200, struct {
			Msg string `json:"msg"`
		}{name + "-started"})
	})
}

package task

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/util/json/http"
	"github.com/hanzoai/commerce/util/template"
)

// Setup handlers for HTTP registered tasks
func SetupRoutes(router zip.Router) {
	// Redirects
	router.Get("/task", func(c *zip.Ctx) error {
		return c.Redirect(301, "/tasks")
	})

	router.Get("/tasks", func(c *zip.Ctx) error {
		return template.Render(c, "tasks.html", "tasks", Names())
	})

	// Show task
	router.Get("/task/:name", func(c *zip.Ctx) error {
		name := c.Param("name")
		return template.Render(c, "task.html", "task", name)
	})

	// Run task
	router.Post("/task/:name", func(c *zip.Ctx) error {
		name := c.Param("name")
		Run(c, name)
		return template.Render(c, "task-running.html", "task", name)
	})

	router.Get("/run-tasks", func(c *zip.Ctx) error {
		return http.Render(c, 200, Names())
	})

	router.Get("/run-task/:name", func(c *zip.Ctx) error {
		name := c.Param("name")
		Run(c, name)
		return c.Redirect(301, "/run-task/"+name+"/started")
	})

	router.Get("/run-task/:name/started", func(c *zip.Ctx) error {
		name := c.Param("name")
		return http.Render(c, 200, struct {
			Msg string `json:"msg"`
		}{name + "-started"})
	})
}

package task

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/util/router"
	"hanzo.io/util/template"
)

// Setup handlers for HTTP registered tasks
func SetupRoutes(router router.Router) {
	// Redirects
	router.GET("/task", func(c *context.Context) {
		c.Redirect(301, "/tasks")
	})

	router.GET("/tasks", func(c *context.Context) {
		template.Render(c, "tasks.html", "tasks", Names())
	})

	// Show task
	router.GET("/task/:name", func(c *context.Context) {
		name := c.Params.ByName("name")
		template.Render(c, "task.html", "task", name)
	})

	// Run task
	router.POST("/task/:name", func(c *context.Context) {
		name := c.Params.ByName("name")
		Run(c, name)
		template.Render(c, "task-running.html", "task", name)
	})
}

package task

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/util/template"
)

func SetupRoutes(router *gin.RouterGroup) {
	// Handler for HTTP registered tasks
	router.GET("/tasks", func(c *gin.Context) {
		template.Render(c, "tasks.html", "tasks", Names())
	})

	router.GET("/task/", func(c *gin.Context) {
		c.Redirect(301, "/tasks")
	})

	router.GET("/task/:name", func(c *gin.Context) {
		name := c.Params.ByName("name")
		template.Render(c, "task.html", "task", name)
	})

	router.POST("/task/:name", func(c *gin.Context) {
		name := c.Params.ByName("name")
		Run(c, name)
		template.Render(c, "task-running.html", "task", name)
	})
}

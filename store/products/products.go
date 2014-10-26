package products

import (
	"github.com/gin-gonic/gin"
	"crowdstart.io/util/template"
)

func List(c *gin.Context) {
	if err := template.Render(c, "products/list.html", nil); err != nil {
		c.String(500, "Unable to render template")
	}
}

func Get(c *gin.Context) {
	slug := c.Params.ByName("slug")

	if err := template.Render(c, "products/" + slug + ".html", nil); err != nil {
		c.String(500, "Unable to render template")
	}
}

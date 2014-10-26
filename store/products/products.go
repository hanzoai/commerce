package products

import (
	"github.com/gin-gonic/gin"
	"crowdstart.io/util/template"
)

func List(c *gin.Context) {
	err := template.Render(c, "products/list.html", nil); if err != nil {
		c.String(500, "Unable to render template")
	}
}

func Get(c *gin.Context) {
	slug := c.Params.ByName("slug")

	err := template.Render(c, slug + ".html", nil); if err != nil {
		c.String(500, "Unable to render template")
	}
}

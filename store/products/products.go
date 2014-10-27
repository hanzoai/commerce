package products

import (
	"github.com/gin-gonic/gin"
	"crowdstart.io/util/template"
)

func List(c *gin.Context) {
	template.Render(c, "products/list.html", nil)
}

func Get(c *gin.Context) {
	slug := c.Params.ByName("slug")
	template.Render(c, "products/" + slug + ".html", nil)
}

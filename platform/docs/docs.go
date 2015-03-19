package docs

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/util/template"
)

func Docs(c *gin.Context) {
	template.Render(c, "docs/docs.html")
}

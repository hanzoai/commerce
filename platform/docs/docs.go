package docs

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/util/template"
)

func Introduction(c *gin.Context) {
	template.Render(c, "docs/introduction.html")
}

func API(c *gin.Context) {
	template.Render(c, "docs/api.html")
}

func CrowdstartJS(c *gin.Context) {
	template.Render(c, "docs/crowdstart.js.html")
}

func Salesforce(c *gin.Context) {
	template.Render(c, "docs/salesforce.html")
}

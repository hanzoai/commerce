package docs

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/util/template"
)

func GettingStarted(c *gin.Context) {
	template.Render(c, "docs/getting-started.html")
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

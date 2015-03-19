package frontend

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/util/template"
)

func Index(c *gin.Context) {
	template.Render(c, "frontend/index.html")
}

func About(c *gin.Context) {
	template.Render(c, "frontend/about.html")
}

func Contact(c *gin.Context) {
	template.Render(c, "frontend/contact.html")
}

func Docs(c *gin.Context) {
	template.Render(c, "frontend/docs.html")
}

func Faq(c *gin.Context) {
	template.Render(c, "frontend/faq.html")
}

func Features(c *gin.Context) {
	template.Render(c, "frontend/features.html")
}

func Pricing(c *gin.Context) {
	template.Render(c, "frontend/pricing.html")
}

func Privacy(c *gin.Context) {
	template.Render(c, "frontend/privacy.html")
}

func Team(c *gin.Context) {
	template.Render(c, "frontend/team.html")
}

func Terms(c *gin.Context) {
	template.Render(c, "frontend/terms.html")
}

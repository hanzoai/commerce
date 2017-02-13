package frontend

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/auth"
	"hanzo.io/util/template"
)

func Render(c *gin.Context, tmpl string) {
	if usr, err := auth.GetCurrentUser(c); err == nil {
		template.Render(c, tmpl, "loggedIn", true, "user", usr)
		return
	}

	template.Render(c, tmpl, "loggedIn", false)
}

func Index(c *gin.Context) {
	Render(c, "frontend/index.html")
}

func About(c *gin.Context) {
	Render(c, "frontend/about.html")
}

func Contact(c *gin.Context) {
	Render(c, "frontend/contact.html")
}

func Docs(c *gin.Context) {
	Render(c, "docs/docs.html")
}

func Faq(c *gin.Context) {
	Render(c, "frontend/faq.html")
}

func Features(c *gin.Context) {
	Render(c, "frontend/features.html")
}

func HowItWorks(c *gin.Context) {
	Render(c, "frontend/how-it-works.html")
}

func Pricing(c *gin.Context) {
	Render(c, "frontend/pricing.html")
}

func Privacy(c *gin.Context) {
	Render(c, "frontend/privacy.html")
}

func Signup(c *gin.Context) {
	Render(c, "frontend/signup.html")
}

func Team(c *gin.Context) {
	Render(c, "frontend/team.html")
}

func Terms(c *gin.Context) {
	Render(c, "frontend/terms.html")
}

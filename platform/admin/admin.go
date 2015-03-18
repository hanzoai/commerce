package admin

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/auth"
	"crowdstart.io/config"
	"crowdstart.io/util/log"
	"crowdstart.io/util/template"
)

// Index
func Index(c *gin.Context) {
	url := config.UrlFor("platform", "/dashboard")
	log.Debug("Redirecting to %s", url)
	c.Redirect(301, url)
}

// // Register
// func Register(c *gin.Context) {
// 	template.Render(c, "register.html")
// }

// // Post registration form
// func SubmitRegister(c *gin.Context) {
// 	c.Redirect(301, "dashboard")
// }

// Render login form
func Login(c *gin.Context) {
	template.Render(c, "login.html")
}

// Post login form
func SubmitLogin(c *gin.Context) {
	if err := auth.VerifyUser(c); err == nil {
		log.Debug("Success")
		c.Redirect(301, "dashboard")
	} else {
		log.Debug("Failure")
		log.Debug("%#v", err)
		template.Render(c, "login.html", "failed", true)
	}
}

//
func Logout(c *gin.Context) {
	auth.Logout(c) // Deletes the loginKey from session.Values
	c.Redirect(301, "/")
}

// Renders the admin user page
func Profile(c *gin.Context) {

}

// Handles submission on profile page
func SubmitProfile(c *gin.Context) {
	c.Redirect(301, "profile")
}

// Admin Dashboard
func Dashboard(c *gin.Context) {
	template.Render(c, "dashboard.html")
}

// Admin Payment Connectors
func Connect(c *gin.Context) {
	template.Render(c, "connect.html",
		"stripe", config.Stripe,
		"salesforce", config.Salesforce)
}

// Theme Testing
func ThemeSample(c *gin.Context) {
	template.Render(c, "sample.html")
}

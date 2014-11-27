package admin

import (
	"log"

	"crowdstart.io/auth"
	"crowdstart.io/config"
	"crowdstart.io/util/template"
	"github.com/gin-gonic/gin"
)

type TokenData struct {
	Access_token           string
	Error                  string
	Error_description      string
	Livemode               bool
	Refresh_token          string
	Scope                  string
	Stripe_publishable_key string
	Stripe_user_id         string
	Token_type             string
}

// Index
func Index(c *gin.Context) {
	c.Redirect(301, config.UrlFor("platform", "/dashboard"))
}

// Register
func Register(c *gin.Context) {
	template.Render(c, "adminlte/register.html")
}

// Post registration form
func SubmitRegister(c *gin.Context) {
	c.Redirect(301, "dashboard")
}

// Render login form
func Login(c *gin.Context) {
	template.Render(c, "adminlte/login.html")
}

// Post login form
func SubmitLogin(c *gin.Context) {
	if err := auth.VerifyUser(c); err == nil {
		log.Println("Success")
		c.Redirect(301, "dashboard")
	} else {
		log.Println("Failure")
		log.Printf("%#v", err)
		c.Redirect(301, "login")
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
	template.Render(c, "adminlte/dashboard.html")
}

// Admin Payment Connectors
func Connect(c *gin.Context) {
	template.Render(c, "adminlte/connect.html", "clientid", config.Get().Stripe.ClientId)
}

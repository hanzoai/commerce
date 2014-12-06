package user

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/auth"
	"crowdstart.io/config"
	"crowdstart.io/util/template"
)

// GET /login
func Login(c *gin.Context) {
	template.Render(c, "store/login.html")
}

// POST /login
func SubmitLogin(c *gin.Context) {
	if err := auth.VerifyUser(c); err == nil {
		c.Redirect(300, config.UrlFor("store", "/profile"))
	} else {
		template.Render(c, "store/login.html",
			"error", "Invalid email or password",
		)
	}
}

// GET /logout
func Logout(c *gin.Context) {
	auth.Logout(c)
	c.Redirect(300, config.UrlFor("store"))
}

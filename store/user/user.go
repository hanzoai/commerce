package user

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/auth"
	"crowdstart.io/config"
	"crowdstart.io/datastore"
	"crowdstart.io/models"
	"crowdstart.io/util/log"
	"crowdstart.io/util/template"
)

// GET /login
func Login(c *gin.Context) {
	template.Render(c, "login.html")
}

// POST /login
func SubmitLogin(c *gin.Context) {
	if err := auth.VerifyUser(c); err == nil {
		c.Redirect(300, config.UrlFor("store", "/profile"))
	} else {
		template.Render(c, "login.html",
			"error", "Invalid email or password",
		)
	}
}

// GET /logout
func Logout(c *gin.Context) {
	auth.Logout(c)
	c.Redirect(300, config.UrlFor("store"))
}

func Register(c *gin.Context) {
	template.Render(c, "register.html")
}

func SubmitRegister(c *gin.Context) {
	f := new(auth.RegistrationForm)
	err := f.Parse(c)
	if err != nil {
		log.Panic("Error parsing user \n%v", err)
	}

	db := datastore.New(c)
	existingUser := new(models.User)
	db.GetKey("user", f.User.Email, existingUser)
	if existingUser != nil {
		template.Render(c, "register.html", "error", "Email has been used already.")
		return
	}

	f.User.Id = f.User.Email
	f.User.PasswordHash, err = f.PasswordHash()
	if err != nil {
		log.Panic("Error generating password hash \n%v", err)
	}

	_, err = db.PutKey("user", f.User.Email, f.User)
	if err != nil {
		log.Panic("Error while saving user \n%v", err)
	}

	err = auth.Login(c, f.User.Email)
	if err != nil {
		log.Panic("Error while setting session cookie %v", err)
	}

	c.Redirect(300, config.UrlFor("store", "/profile"))
}

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
	if auth.IsLoggedIn(c) {
		c.Redirect(302, config.UrlFor("store", "/profile"))
	}
	template.Render(c, "login.html")
}

// POST /login
func SubmitLogin(c *gin.Context) {
	if err := auth.VerifyUser(c); err == nil {
		c.Redirect(302, config.UrlFor("store", "/profile"))
	} else {
		template.Render(c, "login.html",
			"error", "Invalid email or password",
		)
	}
}

// GET /logout
func Logout(c *gin.Context) {
	err := auth.Logout(c)
	if err != nil {
		log.Panic("Error while logging out \n%v", err)
	}
	c.Redirect(302, config.UrlFor("store"))
}

func Register(c *gin.Context) {
	if auth.IsLoggedIn(c) {
		c.Redirect(302, config.UrlFor("store", "/profile"))
	}
	template.Render(c, "register.html")
}

func SubmitRegister(c *gin.Context) {
	f := new(auth.RegistrationForm)
	err := f.Parse(c)
	if err != nil {
		log.Panic("Error parsing user \n%v", err)
	}

	db := datastore.New(c)

	log.Debug("Checking if user exists")
	var existingUser models.User
	err = db.GetKey("user", f.User.Email, &existingUser)
	if err == nil {
		template.Render(c, "register.html", "error", "Email has been used already.")
		return
	}

	f.User.Id = f.User.Email
	f.User.PasswordHash, err = f.PasswordHash()
	if err != nil {
		log.Panic("Error generating password hash \n%v", err)
	}

	log.Debug("Saving user")
	_, err = db.PutKey("user", f.User.Email, &f.User)
	if err != nil {
		log.Panic("Error while saving user \n%v", err)
	}

	log.Debug("Login user")
	err = auth.Login(c, f.User.Email)
	if err != nil {
		log.Panic("Error while setting session cookie %v", err)
	}

	c.Redirect(302, config.UrlFor("store"))
}

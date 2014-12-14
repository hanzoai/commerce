package user

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/auth"
	"crowdstart.io/config"
	"crowdstart.io/datastore"
	"crowdstart.io/models"
	"crowdstart.io/util/log"
	"crowdstart.io/util/template"
	"crowdstart.io/util/val"
)

// GET /login
func Login(c *gin.Context) {
	template.Render(c, "login.html")
}

// POST /login
func SubmitLogin(c *gin.Context) {
	if err := auth.VerifyUser(c); err == nil {
		c.Redirect(302, config.UrlFor("store", "/profile"))
	} else {
		template.Render(c, "login.html", "loginError", "Invalid email or password")
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
	c.Redirect(302, config.UrlFor("store/login"))
	//	template.Render(c, "register.html")
}

func SubmitRegister(c *gin.Context) {
	f := new(auth.RegistrationForm)
	err := f.Parse(c)
	if err != nil {
		template.Render(c, "login.html", "registerError", "An error has occured, please try again later.")
		return
	}

	// Validation
	user := f.User
	log.Debug("Register Validation for %v", user)
	log.Debug("Form is %v", f)
	if !val.Check(user.FirstName).Exists().IsValid {
		log.Debug("Form posted without first name")
		template.Render(c, "login.html", "registerError", "Please enter a first name.")
		return
	}

	if !val.Check(user.LastName).Exists().IsValid {
		log.Debug("Form posted without last name")
		template.Render(c, "login.html", "registerError", "Please enter a last name.")
		return
	}

	if !val.Check(user.Email).IsEmail().IsValid {
		log.Debug("Form posted invalid email")
		template.Render(c, "login.html", "registerError", "Please enter a valid email.")
		return
	}

	if !val.Check(f.Password).IsPassword().IsValid {
		log.Debug("Form posted invalid password")
		template.Render(c, "login.html", "registerError", "Password Must be atleast 6 characters long.")
		return
	}

	db := datastore.New(c)

	log.Debug("Checking if user exists")
	var existingUser models.User
	err = db.GetKey("user", f.User.Email, &existingUser)
	if err == nil {
		template.Render(c, "login.html", "registerError", "An account already exists for this email.")
		return
	}

	f.User.Id = f.User.Email
	f.User.PasswordHash, err = f.PasswordHash()
	if err != nil {
		template.Render(c, "login.html", "registerError", "An error has occured, please try again later.")
		log.Panic("Error generating password hash \n%v", err)
	}

	log.Debug("Saving user")
	_, err = db.PutKey("user", f.User.Email, &f.User)
	if err != nil {
		template.Render(c, "login.html", "registerError", "An error has occured, please try again later.")
		log.Panic("Error while saving user \n%v", err)
	}

	log.Debug("Login user")
	err = auth.Login(c, f.User.Email)
	if err != nil {
		template.Render(c, "login.html", "registerError", "An error has occured, please try again later.")
		log.Panic("Error while setting session cookie %v", err)
	}

	c.Redirect(302, config.UrlFor("store"))
}

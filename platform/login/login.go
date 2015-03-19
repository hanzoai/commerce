package login

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/auth2"
	"crowdstart.io/config"
	"crowdstart.io/util/log"
	"crowdstart.io/util/template"
	"crowdstart.io/util/val"
)

// GET /login
func Login(c *gin.Context) {
	template.Render(c, "login/login.html")
}

// POST /login
func LoginSubmit(c *gin.Context) {
	if _, err := auth.VerifyUser(c); err == nil {
		log.Debug("Success")
		c.Redirect(301, "dashboard")
	} else {
		log.Debug("Failure")
		log.Debug("%#v", err)
		template.Render(c, "login/login.html", "failed", true)
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

func Signup(c *gin.Context) {
	template.Render(c, "login/signup.html")
}

func SignupSubmit(c *gin.Context) {
	form := new(auth.RegistrationForm)
	err := form.Parse(c)
	if err != nil {
		template.Render(c, "login/login.html", "registerError", "An error has occured, please try again later.")
		return
	}

	// Validation
	user := form.User
	log.Debug("Register Validation for %v", user)
	log.Debug("Form is %v", form)
	if !val.Check(user.FirstName).Exists().IsValid {
		log.Debug("Form posted without first name")
		template.Render(c, "login/login.html", "registerError", "Please enter a first name.")
		return
	}

	if !val.Check(user.LastName).Exists().IsValid {
		log.Debug("Form posted without last name")
		template.Render(c, "login/login.html", "registerError", "Please enter a last name.")
		return
	}

	if !val.Check(user.Email).IsEmail().IsValid {
		log.Debug("Form posted invalid email")
		template.Render(c, "login/login.html", "registerError", "Please enter a valid email.")
		return
	}

	if !val.Check(form.Password).IsPassword().IsValid {
		log.Debug("Form posted invalid password")
		template.Render(c, "login/login.html", "registerError", "Password Must be atleast 6 characters long.")
		return
	}

	// Santitization
	val.SanitizeUser2(&form.User)

	// _, err = auth.NewUser(c, f)
	// if err != nil && err.Error() == "Email is already registered" {
	// 	template.Render(c, "login/login.html", "registerError", "An account already exists for this email.")
	// 	return
	// }

	// if err != nil {
	// 	template.Render(c, "login/login.html", "registerError", "An error has occured, please try again later.")
	// 	log.Panic("Error generating password hash \n%v", err)
	// }

	// log.Debug("Login user")
	// err = auth.Login(c, f.User.Email)
	// if err != nil {
	// 	template.Render(c, "login/login.html", "registerError", "An error has occured, please try again later.")
	// 	log.Panic("Error while setting session cookie %v", err)
	// }

	// Look up campaign to see if we need to sync with salesforce
	// db := datastore.New(c)
	// campaign := models.Campaign{}
	// if err := db.GetKind("campaign", "dev@hanzo.ai", &campaign); err != nil {
	// 	log.Error(err, c)
	// }

	// if campaign.Salesforce.AccessToken != "" {
	// 	salesforce.CallUpsertUserTask(db.Context, &campaign, u)
	// }
	c.Redirect(302, config.UrlFor("platform"))
}

package user

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/auth"
	"crowdstart.io/config"
	"crowdstart.io/datastore"
	"crowdstart.io/middleware"
	"crowdstart.io/models"
	"crowdstart.io/thirdparty/mandrill"
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
		c.Redirect(302, config.UrlFor("store", "/profile"))
	} else {
		template.Render(c, "login.html", "error", "Invalid email or password")
	}
}

// GET /forgotpassword
func ForgotPassword(c *gin.Context) {
	template.Render(c, "forgotpassword.html")
}

// POST /forgotpassword
func ForgotPassword(c *gin.Context) {
	ctx := middleware.GetAppEngine(c)

	form, err := ForgotPasswordForm(c)
	if err != nil {
		template.Render(c, "forgotpassword.html",
			"error", "Please enter your email.")
		return
	}

	db := datastore.New(ctx)
	var user models.User
	err := db.GetKey("user", form.Email, &user)
	if err != nil {
		template.Render(c, "forgotpassword.html",
			"error", "No account associated with that email.")
		return
	}

	mandrill.SendTemplateAsync(ctx, "forgotten-password", user.Email, user.Name())

	template.Render(c, "forgotpassword-email-sent.html")
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

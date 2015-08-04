package user

import (
	"errors"

	"github.com/gin-gonic/gin"

	"crowdstart.com/auth"
	"crowdstart.com/auth/password"
	"crowdstart.com/config"
	"crowdstart.com/datastore"
	"crowdstart.com/middleware"
	"crowdstart.com/models/organization"
	"crowdstart.com/util/log"
	"crowdstart.com/util/template"
)

var ErrorInvalidProfile = errors.New("Invalid Profile Saved")
var ErrorPasswordIncorrect = errors.New("Password Incorrect")
var ErrorPasswordTooShort = errors.New("Password must be atleast 6 characters long")

// Renders the profile page
func Profile(c *gin.Context) {
	Render(c, "admin/profile.html")
}

// Handles submission on profile page
func ContactSubmit(c *gin.Context) {
	form := new(ContactForm)
	if err := form.Parse(c); err != nil {
		log.Panic("Failed to save user profile: %v", err)
	}

	// val.SanitizeUser2(&form.User)
	if errs := form.Validate(); len(errs) > 0 {
		c.AbortWithError(500, ErrorInvalidProfile)
		return
	}

	u := middleware.GetCurrentUser(c)

	// Update information from form.
	u.FirstName = form.User.FirstName
	u.LastName = form.User.LastName
	u.Email = form.User.Email
	u.Phone = form.User.Phone

	u.Put()

	c.Redirect(301, config.UrlFor("platform/", "profile"))
}

func PasswordSubmit(c *gin.Context) {
	form := new(ChangePasswordForm)
	if err := form.Parse(c); err != nil {
		log.Panic("Failed to update user password: %v", err)
	}

	u := middleware.GetCurrentUser(c)

	if !password.HashAndCompare(u.PasswordHash, form.OldPassword) {
		log.Debug("Old password is incorrect.")
		c.AbortWithError(500, ErrorPasswordIncorrect)
		return
	}

	if form.Password == form.ConfirmPassword {
		if errs := form.Validate(); len(errs) > 0 {
			c.AbortWithError(500, ErrorPasswordTooShort)
			return
		}

		var err error
		if u.PasswordHash, err = password.Hash(form.Password); err != nil {
			c.AbortWithError(500, err)
			return
		}

		u.Put()

		c.Redirect(301, config.UrlFor("platform/", "profile"))
	} else {
		log.Debug("Passwords do not match.")
		c.AbortWithError(500, auth.ErrorPasswordMismatch)
		return
	}
}

func Render(c *gin.Context, name string, args ...interface{}) {
	db := datastore.New(c)
	org := organization.New(db)
	if err := org.GetById("crowdstart"); err == nil {
		args = append(args, "crowdstartId", org.Id())
	} else {
		args = append(args, "crowdstartId", "")
	}
	log.Warn("Z%s", org.Id())

	template.Render(c, name, args...)
}

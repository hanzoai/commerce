package user

import (
	"errors"

	"github.com/gin-gonic/gin"

	"crowdstart.io/auth"
	"crowdstart.io/auth/password"
	"crowdstart.io/config"
	"crowdstart.io/middleware"
	"crowdstart.io/util/log"
	"crowdstart.io/util/template"
)

var ErrorInvalidProfile = errors.New("Invalid Profile Saved")
var ErrorPasswordIncorrect = errors.New("Password Incorrect")
var ErrorPasswordTooShort = errors.New("Password must be atleast 6 characters long")

// Renders the profile page
func Profile(c *gin.Context) {
	template.Render(c, "admin/profile.html")
}

// Handles submission on profile page
func ContactSubmit(c *gin.Context) {
	form := new(ContactForm)
	if err := form.Parse(c); err != nil {
		log.Panic("Failed to save user profile: %v", err)
	}

	// val.SanitizeUser2(&form.User)
	if errs := form.Validate(); len(errs) > 0 {
		c.Fail(500, ErrorInvalidProfile)
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
		c.Fail(500, ErrorPasswordIncorrect)
		return
	}

	if form.Password == form.ConfirmPassword {
		if errs := form.Validate(); len(errs) > 0 {
			c.Fail(500, ErrorPasswordTooShort)
			return
		}

		var err error
		if u.PasswordHash, err = password.Hash(form.Password); err != nil {
			c.Fail(500, err)
			return
		}

		u.Put()

		c.Redirect(301, config.UrlFor("platform/", "profile"))
	} else {
		log.Debug("Passwords do not match.")
		c.Fail(500, auth.ErrorPasswordMismatch)
		return
	}
}

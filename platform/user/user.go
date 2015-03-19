package user

import (
	"errors"

	"github.com/gin-gonic/gin"

	"crowdstart.io/auth2"
	"crowdstart.io/auth2/password"
	"crowdstart.io/util/log"
	"crowdstart.io/util/template"
	"crowdstart.io/util/val"
)

var ErrorInvalidProfile = errors.New("Invalid Profile Saved")
var ErrorPasswordIncorrect = errors.New("Password Incorrect")
var ErrorPasswordTooShort = errors.New("Password must be atleast 6 characters long")

// Render login form
func Login(c *gin.Context) {
	template.Render(c, "login/login.html")
}

// Post login form
func SubmitLogin(c *gin.Context) {
	if _, err := auth.VerifyUser(c); err == nil {
		log.Debug("Success")
		c.Redirect(301, "dashboard")
	} else {
		log.Debug("Failure")
		log.Debug("%#v", err)
		template.Render(c, "login/login.html", "failed", true)
	}
}

// Log user out
func Logout(c *gin.Context) {
	auth.Logout(c) // Deletes the loginKey from session.Values
	c.Redirect(301, "/")
}

// Renders the profile page
func Profile(c *gin.Context) {
	if u, err := auth.GetCurrentUser(c); err != nil {
		c.Fail(500, err)
		return
	} else {
		template.Render(c, "profile.html",
			"user", u)
	}
}

// Handles submission on profile page
func SubmitProfile(c *gin.Context) {
	if u, err := auth.GetCurrentUser(c); err != nil {
		c.Fail(500, err)
		return
	} else {
		form := new(ContactForm)
		if err := form.Parse(c); err != nil {
			log.Panic("Failed to save user profile: %v", err)
		}

		val.SanitizeUser2(&form.User)
		if errs := form.Validate(); len(errs) > 0 {
			c.Fail(500, ErrorInvalidProfile)
			return
		}

		// Update information from form.
		u.FirstName = form.User.FirstName
		u.LastName = form.User.LastName
		u.Email = form.User.Email
		u.Phone = form.User.Phone

		u.Put()

		c.Redirect(301, "profile")
	}
}

func ResetPassword(c *gin.Context) {
	if u, err := auth.GetCurrentUser(c); err != nil {
		c.Fail(500, err)
		return
	} else {
		form := new(ChangePasswordForm)
		if err := form.Parse(c); err != nil {
			log.Panic("Failed to update user password: %v", err)
		}

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

			if u.PasswordHash, err = password.Hash(form.Password); err != nil {
				c.Fail(500, err)
				return
			}

			u.Put()

			c.Redirect(301, "profile")
		} else {
			log.Debug("Passwords do not match.")
			c.Fail(500, auth.ErrorPasswordMismatch)
			return
		}
	}
}

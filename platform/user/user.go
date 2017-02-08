package user

import (
	"errors"

	"github.com/gin-gonic/gin"

	"hanzo.io/auth"
	"hanzo.io/auth/password"
	"hanzo.io/middleware"
	"hanzo.io/util/json/http"
)

var ErrorInvalidProfile = errors.New("Invalid Profile Saved")
var ErrorPasswordIncorrect = errors.New("Password Incorrect")
var ErrorPasswordTooShort = errors.New("Password must be atleast 6 characters long")

// Renders the profile page
func Profile(c *gin.Context) {
	u := middleware.GetCurrentUser(c)
	http.Render(c, 200, u)
}

// Handles submission on profile page
func ContactSubmit(c *gin.Context) {
	form := new(ContactForm)
	if err := form.Parse(c); err != nil {
		http.Fail(c, 400, "Failed decode request body", err)
		return
	}

	u := middleware.GetCurrentUser(c)

	// Update information from form.
	u.FirstName = form.User.FirstName
	u.LastName = form.User.LastName
	u.Email = form.User.Email
	u.Phone = form.User.Phone

	u.Put()

	http.Render(c, 200, u)
}

func PasswordSubmit(c *gin.Context) {
	form := new(ChangePasswordForm)
	if err := form.Parse(c); err != nil {
		http.Fail(c, 400, "Failed decode request body", err)
		return
	}

	u := middleware.GetCurrentUser(c)

	if !password.HashAndCompare(u.PasswordHash, form.OldPassword) {
		http.Fail(c, 500, ErrorPasswordIncorrect.Error(), ErrorPasswordIncorrect)
		return
	}

	if form.Password == form.ConfirmPassword {
		if errs := form.Validate(); len(errs) > 0 {
			http.Fail(c, 500, ErrorPasswordTooShort.Error(), ErrorPasswordTooShort)
			return
		}

		var err error
		if u.PasswordHash, err = password.Hash(form.Password); err != nil {
			http.Fail(c, 500, "Something has gone wrong.", err)
			return
		}

		u.Put()
		c.Writer.WriteHeader(204)
	} else {
		http.Fail(c, 500, auth.ErrorPasswordMismatch.Error(), auth.ErrorPasswordMismatch)
		return
	}
}

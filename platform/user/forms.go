package user

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/models/user"
	"crowdstart.com/util/form"
	"crowdstart.com/util/val"
)

// User profile form (contact)
type ContactForm struct {
	User user.User
}

func (f *ContactForm) Parse(c *gin.Context) error {
	return form.Parse(c, f)
}

func (f *ContactForm) Validate() []string {
	var errs []string
	// errs = val.ValidateUser2(&f.User, errs)
	return errs
}

// User profile form (change password)
type ChangePasswordForm struct {
	OldPassword     string
	Password        string
	ConfirmPassword string
}

func (f *ChangePasswordForm) Parse(c *gin.Context) error {
	return form.Parse(c, f)
}

func (f *ChangePasswordForm) Validate() []string {
	var errs []string
	errs = val.ValidatePassword(f.Password, errs)
	return errs
}

// Reset Password form (request)
type ResetPasswordForm struct {
	Email string
}

func (f *ResetPasswordForm) Parse(c *gin.Context) error {
	return form.Parse(c, f)
}

// Reset Password form (confirm)
type ResetPasswordConfirmForm struct {
	NewPassword     string
	ConfirmPassword string
}

func (f *ResetPasswordConfirmForm) Parse(c *gin.Context) error {
	return form.Parse(c, f)
}

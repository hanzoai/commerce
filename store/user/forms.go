package user

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/models"
	"crowdstart.io/util/form"
)

// User profile form (contact)
type ContactForm struct {
	User models.User
}

func (f *ContactForm) Parse(c *gin.Context) error {
	return form.Parse(c, f)
}

// User profile form (billing)
type BillingForm struct {
	BillingAddress models.Address
}

func (f *BillingForm) Parse(c *gin.Context) error {
	return form.Parse(c, f)
}

// User profile form (change password)
type ChangePasswordForm struct {
	Password        string
	NewPassword     string
	ConfirmPassword string
}

func (f *ChangePasswordForm) Parse(c *gin.Context) error {
	return form.Parse(c, f)
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

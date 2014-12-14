package user

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/models"
	"crowdstart.io/util/form"
)

// Forgotten Password form
type ForgotPasswordForm struct {
	Email string
}

func (f *ForgotPasswordForm) Parse(c *gin.Context) error {
	return form.Parse(c, f)
}

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
	OldPassword     string
	Password        string
	ConfirmPassword string
}

func (f *ChangePasswordForm) Parse(c *gin.Context) error {
	return form.Parse(c, f)
}

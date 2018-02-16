package login

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"

	"hanzo.io/auth/password"
	"hanzo.io/models"
	"hanzo.io/models/user"
	"hanzo.io/util/form"
	"hanzo.io/util/val"
)

type LoginForm struct {
	Email    string
	Password string
}

func (f LoginForm) PasswordHashAndCompare(hash []byte) bool {
	return password.HashAndCompare(hash, f.Password)
}

func (f LoginForm) PasswordHash() ([]byte, error) {
	return password.Hash(f.Password)
}

func (f *LoginForm) Parse(c *context.Context) error {
	if err := form.Parse(c, f); err != nil {
		return err
	}

	f.Email = strings.TrimSpace(strings.ToLower(f.Email))

	return nil
}

// User profile form (contact)
type ContactForm struct {
	User *user.User
}

func (f *ContactForm) Parse(c *context.Context) error {
	return form.Parse(c, f)
}

func (f *ContactForm) Validate() []string {
	var errs []string
	// errs = val.ValidateUser(&f.User, errs)
	return errs
}

// User profile form (billing)
type BillingForm struct {
	BillingAddress models.Address
}

func (f *BillingForm) Parse(c *context.Context) error {
	return form.Parse(c, f)
}

func (f *BillingForm) Validate() []string {
	var errs []string
	// errs = val.ValidateAddress(&f.BillingAddress, errs)
	return errs
}

// User profile form (change password)
type ChangePasswordForm struct {
	OldPassword     string
	Password        string
	ConfirmPassword string
}

func (f *ChangePasswordForm) Parse(c *context.Context) error {
	return form.Parse(c, f)
}

func (f *ChangePasswordForm) Validate() []string {
	var errs []string
	errs = val.ValidatePassword(f.Password, errs)
	return errs
}

// Reset Password form (request)
type PasswordResetForm struct {
	Email string
}

func (f *PasswordResetForm) Parse(c *context.Context) error {
	return form.Parse(c, f)
}

// Reset Password form (confirm)
type PasswordResetConfirmForm struct {
	NewPassword     string
	ConfirmPassword string
}

func (f *PasswordResetConfirmForm) Parse(c *context.Context) error {
	return form.Parse(c, f)
}

// SignupForm
type SignupForm struct {
	Email           string
	Password        string
	ConfirmPassword string
}

func (f *SignupForm) Parse(c *context.Context) error {
	if err := form.Parse(c, f); err != nil {
		return err
	}

	// Clean up any trailing spaces
	f.Password = strings.Trim(f.Password, " ")
	f.ConfirmPassword = strings.Trim(f.ConfirmPassword, " ")
	return nil
}

func (f *SignupForm) Validate() []error {
	errs := make([]error, 0)
	if f.Password != f.ConfirmPassword {
		errs = append(errs, errors.New("Passwords do not match"))
	}

	return errs
}

func (f SignupForm) PasswordHash() []byte {
	hash, err := password.Hash(f.Password)
	if err != nil {
		panic(err)
	}
	return hash
}

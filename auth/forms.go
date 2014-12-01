package auth

import (
	"strings"

	"code.google.com/p/go.crypto/bcrypt"
	"github.com/gin-gonic/gin"

	"crowdstart.io/models"
	"crowdstart.io/util/form"
)

type LoginForm struct {
	Email    string
	Password string
}

func (f *LoginForm) PasswordHash() ([]byte, error) {
	return bcrypt.GenerateFromPassword([]byte(f.Password), 12)
}

func (f *LoginForm) Parse(c *gin.Context) error {
	if err := form.Parse(c, f); err != nil {
		return err
	}

	f.Email = strings.ToLower(f.Email)

	return nil
}

type RegistrationForm struct {
	User     models.User
	Password string
}

func (f *RegistrationForm) Parse(c *gin.Context) error {
	return form.Parse(c, f)
}

func (f *RegistrationForm) PasswordHash() ([]byte, error) {
	return bcrypt.GenerateFromPassword([]byte(f.Password), 12)
}

package models

import (
	"crowdstart.io/util/form"
	"github.com/gin-gonic/gin"
	"code.google.com/p/go.crypto/bcrypt"
)

type LoginForm struct {
	Email    string
	Password string
}

func (f LoginForm) PasswordHash() ([]byte, error) {
	return bcrypt.GenerateFromPassword([]byte(f.Password), 12)
}

func (f *LoginForm) Parse(c *gin.Context) error {
	return form.Parse(c, f)
}

type RegistrationForm struct {
	User     User
	Password string
}

func (f RegistrationForm) PasswordHash() ([]byte, error) {
	return bcrypt.GenerateFromPassword([]byte(f.Password), 12)
}

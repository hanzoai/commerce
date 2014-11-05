package models

import (
	"code.google.com/p/go.crypto/bcrypt"
	"crowdstart.io/util/form"
	"github.com/gin-gonic/gin"
)

type LoginForm struct {
	Email    string
	Password string
}

const Cost = 12

func (f LoginForm) PasswordHash() ([]byte, error) {
	return bcrypt.GenerateFromPassword([]byte(f.Password), Cost)
}

func (f *LoginForm) Parse(c *gin.Context) error {
	return form.Parse(c, f)
}

type RegistrationForm struct {
	User     User
	Admin    Admin
	Password string
}

func (f RegistrationForm) PasswordHash() ([]byte, error) {
	return bcrypt.GenerateFromPassword([]byte(f.Password), Cost)
}

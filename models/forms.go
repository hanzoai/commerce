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

const Cost = 12

func (f LoginForm) PasswordHash() ([]byte, error) {
	return bcrypt.GenerateFromPassword([]byte(f.Password), Cost)
}

func (f *LoginForm) Parse(c *gin.Context) error {
	return form.Parse(c, f)
}

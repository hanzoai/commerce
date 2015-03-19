package auth

import (
	"strings"

	"github.com/gin-gonic/gin"

	"crowdstart.io/auth2/password"
	"crowdstart.io/models2/user"
	"crowdstart.io/util/form"
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

func (f *LoginForm) Parse(c *gin.Context) error {
	if err := form.Parse(c, f); err != nil {
		return err
	}

	f.Email = strings.TrimSpace(strings.ToLower(f.Email))

	return nil
}

type RegistrationForm struct {
	User     user.User
	Password string
}

func (f *RegistrationForm) Parse(c *gin.Context) error {
	return form.Parse(c, f)
}

func (f RegistrationForm) PasswordHash() ([]byte, error) {
	return password.Hash(f.Password)
}

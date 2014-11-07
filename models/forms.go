package models

import (
	"code.google.com/p/go.crypto/pbkdf2"
	"crowdstart.io/util/form"
	"github.com/gin-gonic/gin"
	"crypto/sha256"
)

type LoginForm struct {
	Email    string
	Password string
}

const salt = "apOWE0I 1 4E04148408B 4 ['"

func (f LoginForm) PasswordHash() []byte {
	return pbkdf2.Key([]byte(f.Password), []byte(salt), 4096, sha256.Size, sha256.New)
}

func (f *LoginForm) Parse(c *gin.Context) error {
	return form.Parse(c, f)
}

type RegistrationForm struct {
	User     User
	Password string
}

func (f RegistrationForm) PasswordHash() []byte {
	return pbkdf2.Key([]byte(f.Password), []byte(salt), 4096, sha256.Size, sha256.New)
}

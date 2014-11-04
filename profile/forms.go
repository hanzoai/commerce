package profile

import (
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

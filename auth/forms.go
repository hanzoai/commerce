package auth

import (
	"strings"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/auth/password"
	"github.com/hanzoai/commerce/util/form"
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

func (f *LoginForm) Parse(c *zip.Ctx) error {
	if err := form.Parse(c, f); err != nil {
		return err
	}

	f.Email = strings.TrimSpace(strings.ToLower(f.Email))

	return nil
}

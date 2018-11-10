package user

import (
	"strings"

	"hanzo.io/util/crypto/md5"
	"hanzo.io/util/webhook"

	. "hanzo.io/types"
)

// Hooks
func (u *User) BeforeCreate() error {
	u.Username = strings.ToLower(u.Username)
	u.Email = strings.ToLower(u.Email)
	u.KYC.Status = KYCStatusInitiated
	return nil
}

func (u *User) BeforeUpdate(prev *User) error {
	u.Username = strings.ToLower(u.Username)
	u.Email = strings.ToLower(u.Email)

	var hashed []string
	for _, v := range u.KYC.Documents {
		hashed = append(hashed, md5.Hash(v))
	}
	u.KYC.Documents = hashed

	return nil
}

func (u *User) AfterCreate() error {
	webhook.Emit(u.Context(), u.Namespace(), "user.created", u)

	return nil
}

func (u *User) AfterUpdate(prev *User) error {
	webhook.Emit(u.Context(), u.Namespace(), "user.updated", u)
	return nil
}

func (u *User) AfterDelete() error {
	webhook.Emit(u.Context(), u.Namespace(), "user.deleted", u)

	return nil
}

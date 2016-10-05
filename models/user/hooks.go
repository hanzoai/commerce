package user

import (
	"strings"

	"crowdstart.com/util/webhook"
)

// Hooks
func (u *User) BeforeCreate() error {
	u.Username = strings.ToLower(u.Username)
	u.Email = strings.ToLower(u.Email)
	return nil
}

func (u *User) AfterCreate() error {
	webhook.Emit(u.Context(), u.Namespace(), "user.created", u)

	u.Increment()
	u.IncrementDay()
	u.IncrementHour()
	u.IncrementMonth()

	return nil
}

func (u *User) AfterUpdate(previous *User) error {
	webhook.Emit(u.Context(), u.Namespace(), "user.updated", u)
	return nil
}

func (u *User) AfterDelete() error {
	webhook.Emit(u.Context(), u.Namespace(), "user.deleted", u)
	return nil
}

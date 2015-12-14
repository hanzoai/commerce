package user

import "crowdstart.com/util/webhook"

// Hooks
func (u *User) AfterCreate() error {
	webhook.Emit(u.Context(), u.Namespace(), "user.created", u)

	u.Increment()
	u.IncrementDay()
	u.IncrementHour()
	u.IncrementMonth()

	return nil
}

func (u *User) AfterUpdate() error {
	webhook.Emit(u.Context(), u.Namespace(), "user.updated", u)
	return nil
}

func (u *User) AfterDelete() error {
	webhook.Emit(u.Context(), u.Namespace(), "user.deleted", u)
	return nil
}

package user

import (
	"time"

	"crowdstart.com/util/webhook"

	counter "crowdstart.com/util/counter2"
)

// Hooks
func (u *User) AfterCreate() error {
	webhook.Emit(u.Context(), u.Namespace(), "user.created", u)

	counter.Increment(u.Context(), u.Kind())
	counter.IncrementDay(u.Context(), u.Kind(), time.Now())
	counter.IncrementHour(u.Context(), u.Kind(), time.Now())
	counter.IncrementMonth(u.Context(), u.Kind(), time.Now())

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

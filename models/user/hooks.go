package user

import (
	"hanzo.io/util/counter"
	"hanzo.io/util/event"
)

// Hooks
// Only fires when user is not in default namespace
func (u *User) AfterCreate() error {
	if u.Namespace() == "" {
		return nil
	}

	// Increment user totals
	ctx := u.Context()
	key := u.Kind()
	now := u.CreatedAt

	counter.Increment(ctx, key)
	counter.IncrementHour(ctx, key, now)
	counter.IncrementDay(ctx, key, now)
	counter.IncrementMonth(ctx, key, now)

	return event.Emit(ctx, u.Namespace(), "user.created", u)
}

func (u *User) AfterUpdate(previous *User) error {
	if u.Namespace() == "" {
		return nil
	}

	return event.Emit(u.Context(), u.Namespace(), "user.updated", u)
}

func (u *User) AfterDelete() error {
	if u.Namespace() == "" {
		return nil
	}

	return event.Emit(u.Context(), u.Namespace(), "user.deleted", u)
}

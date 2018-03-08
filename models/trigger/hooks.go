package trigger

import "hanzo.io/util/event"

// Hooks
func (t *Trigger) AfterCreate() error {
	return event.Emit(t.Context(), t.Namespace(), "trigger.created", t)
}

func (t *Trigger) AfterUpdate(previous *Trigger) error {
	return event.Emit(t.Context(), t.Namespace(), "trigger.updated", t)
}

func (t *Trigger) AfterDelete() error {
	return event.Emit(t.Context(), t.Namespace(), "trigger.deleted", t)
}

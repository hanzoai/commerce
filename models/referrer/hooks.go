package referrer

import "hanzo.io/util/event"

// Hooks
func (r *Referrer) AfterCreate() error {
	return event.Emit(r.Context(), r.Namespace(), "referrer.created", r)
}

func (r *Referrer) AfterUpdate(previous *Referrer) error {
	return event.Emit(r.Context(), r.Namespace(), "referrer.updated", r)
}

func (r *Referrer) AfterDelete() error {
	return event.Emit(r.Context(), r.Namespace(), "referrer.deleted", r)
}

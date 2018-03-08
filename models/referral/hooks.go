package referral

import "hanzo.io/util/event"

// Hooks
func (r *Referral) AfterCreate() error {
	return event.Emit(r.Context(), r.Namespace(), "referral.created", r)
}

func (r *Referral) AfterUpdate(previous *Referral) error {
	return event.Emit(r.Context(), r.Namespace(), "referral.updated", r)
}

func (r *Referral) AfterDelete() error {
	return event.Emit(r.Context(), r.Namespace(), "referral.deleted", r)
}

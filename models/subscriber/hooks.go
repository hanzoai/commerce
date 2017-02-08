package subscriber

import (
	"hanzo.io/util/webhook"
)

// Hooks
func (s *Subscriber) BeforeCreate() error {
	webhook.Emit(s.Context(), s.Namespace(), "subscriber.created", s)

	s.Normalize()

	return nil
}

func (s *Subscriber) BeforeUpdate(previous *Subscriber) error {
	webhook.Emit(s.Context(), s.Namespace(), "subscriber.updated", s)

	s.Normalize()

	return nil
}

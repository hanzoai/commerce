package subscriber

import (
	"hanzo.io/util/counter"
	"hanzo.io/util/event"
)

// Hooks
func (s *Subscriber) BeforeCreate() error {
	s.Normalize()
	return nil
}

func (s *Subscriber) BeforeUpdate(previous *Subscriber) error {
	s.Normalize()
	return nil
}

// Hooks
func (s *Subscriber) AfterCreate() error {
	ctx := s.Context()
	kind := s.Kind()

	counter.Increment(ctx, kind)
	counter.IncrementHour(ctx, kind, s.CreatedAt)
	counter.IncrementDay(ctx, kind, s.CreatedAt)
	counter.IncrementMonth(ctx, kind, s.CreatedAt)

	return event.Emit(ctx, s.Namespace(), "subscriber.created", s)
}

func (s *Subscriber) AfterUpdate(previous *Subscriber) error {
	return event.Emit(s.Context(), s.Namespace(), "subscriber.updated", s)
}

func (s *Subscriber) AfterDelete() error {
	return event.Emit(s.Context(), s.Namespace(), "subscriber.deleted", s)
}

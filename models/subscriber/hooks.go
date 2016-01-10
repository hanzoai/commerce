package subscriber

import (
	"crowdstart.com/util/strings"
	"crowdstart.com/util/webhook"
)

// Hooks
func (s *Subscriber) BeforeCreate() error {
	webhook.Emit(s.Context(), s.Namespace(), "subscriber.created", s)

	s.Email = strings.StripWhitespace(s.Email)

	return nil
}

func (s *Subscriber) BeforeUpdate(previous *Subscriber) error {
	webhook.Emit(s.Context(), s.Namespace(), "subscriber.updated", s)

	s.Email = strings.StripWhitespace(s.Email)

	return nil
}

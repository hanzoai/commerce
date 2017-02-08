package submission

import "hanzo.io/util/event"

// Hooks
func (s *Submission) AfterCreate() error {
	return event.Emit(s.Context(), s.Namespace(), "submission.created", s)
}

func (s *Submission) AfterUpdate(previous *Submission) error {
	return event.Emit(s.Context(), s.Namespace(), "submission.updated", s)
}

func (s *Submission) AfterDelete() error {
	return event.Emit(s.Context(), s.Namespace(), "submission.deleted", s)
}

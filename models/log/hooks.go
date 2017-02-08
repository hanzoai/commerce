package log

import "hanzo.io/util/event"

func (l *Log) AfterCreate() error {
	return event.Emit(l.Context(), l.Namespace(), "log.created", l)
}

func (l *Log) AfterUpdate(previous *Log) error {
	return event.Emit(l.Context(), l.Namespace(), "log.updated", l)
}

func (l *Log) AfterDelete() error {
	return event.Emit(l.Context(), l.Namespace(), "log.deleted", l)
}

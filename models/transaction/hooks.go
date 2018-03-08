package transaction

import "hanzo.io/util/event"

// Hooks
func (t *Transaction) AfterCreate() error {
	return event.Emit(t.Context(), t.Namespace(), "transaction.created", t)
}

func (t *Transaction) AfterUpdate(previous *Transaction) error {
	return event.Emit(t.Context(), t.Namespace(), "transaction.updated", t)
}

func (t *Transaction) AfterDelete() error {
	return event.Emit(t.Context(), t.Namespace(), "transaction.deleted", t)
}

package collection

import "hanzo.io/util/event"

// Hooks
func (c *Collection) AfterCreate() error {
	return event.Emit(c.Context(), c.Namespace(), "collection.created", c)
}

func (c *Collection) AfterUpdate(previous *Collection) error {
	return event.Emit(c.Context(), c.Namespace(), "collection.updated", c)
}

func (c *Collection) AfterDelete() error {
	return event.Emit(c.Context(), c.Namespace(), "collection.deleted", c)
}

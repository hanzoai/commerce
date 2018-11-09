package disclosure

import (
	"hanzo.io/util/crypto/md5"
	"hanzo.io/util/webhook"
)

// Hooks

func (d *Disclosure) BeforeUpdate(prev *Disclosure) error {
	d.Publication = md5.Hash(d.Publication)

	return nil
}

func (d *Disclosure) AfterCreate() error {
	webhook.Emit(d.Context(), d.Namespace(), "disclosure.created", d)

	return nil
}

func (d *Disclosure) AfterUpdate(prev *Disclosure) error {
	webhook.Emit(d.Context(), d.Namespace(), "user.updated", d)
	return nil
}

func (d *Disclosure) AfterDelete() error {
	webhook.Emit(d.Context(), d.Namespace(), "user.deleted", d)

	return nil
}

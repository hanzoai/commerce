package disclosure

import (
	"hanzo.io/util/crypto/sha256"
	"hanzo.io/util/webhook"
)

// Hooks

func (d *Disclosure) BeforeUpdate(prev *Disclosure) error {
	d.Hash = sha256.Hash(d.Publication + d.Type + d.Receiver + d.CreatedAt.String())

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

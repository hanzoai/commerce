package variant

import (
	"hanzo.io/config"
	"hanzo.io/thirdparty/cloudflare"
	"hanzo.io/util/event"
	"hanzo.io/util/log"
)

// Hooks
func (v *Variant) AfterCreate() error {
	return event.Emit(v.Context(), v.Namespace(), "variant.created", v)
}

func (v *Variant) AfterUpdate(previous *Variant) error {
	// Strictly speaking, we should purge this variant from all available stores, but we're not sure of the best way to do that right now.
	// So that's a to-do.
	url := config.UrlFor("api", "/variant/", v.Id())
	if err := cloudflare.Purge(v.Context(), url); err != nil {
		log.Error("Failed to purge variant %v", err, v.Context())
	}
	return event.Emit(v.Context(), v.Namespace(), "variant.updated", v)
}

func (v *Variant) AfterDelete() error {
	// Strictly speaking, we should purge this variant from all available stores, but we're not sure of the best way to do that right now.
	// So that's a to-do.
	url := config.UrlFor("api", "/variant/", v.Id())
	if err := cloudflare.Purge(v.Context(), url); err != nil {
		log.Error("Failed to purge variant %v", err, v.Context())
	}
	return event.Emit(v.Context(), v.Namespace(), "variant.deleted", v)
}

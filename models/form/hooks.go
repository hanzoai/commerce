package form

import (
	"hanzo.io/config"
	"hanzo.io/thirdparty/cloudflare"
	"hanzo.io/util/event"
	"hanzo.io/util/log"
)

// Hooks
func (m *Form) AfterCreate() error {
	return event.Emit(m.Context(), m.Namespace(), "form.created", m)
}

func (m *Form) AfterUpdate(previous *Form) error {
	url := config.UrlFor("api", "/form/", m.Id(), "js")
	if err := cloudflare.Purge(m.Context(), url); err != nil {
		log.Error("Failed to purge form %v", err, m.Context())
	}
	return event.Emit(m.Context(), m.Namespace(), "form.updated", m)
}

func (m *Form) AfterDelete() error {
	url := config.UrlFor("api", "/form/", m.Id(), "js")
	if err := cloudflare.Purge(m.Context(), url); err != nil {
		log.Error("Failed to purge form %v", err, m.Context())
	}
	return event.Emit(m.Context(), m.Namespace(), "form.deleted", m)
}

package bundle

import (
	"hanzo.io/config"
	"hanzo.io/thirdparty/cloudflare"
	"hanzo.io/util/event"
	"hanzo.io/util/log"
)

// Hooks
func (s *Bundle) AfterCreate() error {
	return event.Emit(s.Context(), s.Namespace(), "bundle.created", s)
}

func (s *Bundle) AfterUpdate(previous *Bundle) error {
	url := config.UrlFor("api", "/bundle/", s.Id())
	if err := cloudflare.Purge(s.Context(), url); err != nil {
		log.Error("Failed to purge bundle %v", err, s.Context())
	}
	return event.Emit(s.Context(), s.Namespace(), "bundle.updated", s)
}

func (s *Bundle) AfterDelete() error {
	url := config.UrlFor("api", "/bundle/", s.Id())
	if err := cloudflare.Purge(s.Context(), url); err != nil {
		log.Error("Failed to purge bundle %v", err, s.Context())
	}
	return event.Emit(s.Context(), s.Namespace(), "bundle.deleted", s)
}

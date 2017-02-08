package store

import (
	"hanzo.io/config"
	"hanzo.io/thirdparty/cloudflare"
	"hanzo.io/util/event"
	"hanzo.io/util/log"
)

// Hooks
func (s *Store) AfterCreate() error {
	return event.Emit(s.Context(), s.Namespace(), "store.created", s)
}

func (s *Store) AfterUpdate(previous *Store) error {
	url := config.UrlFor("api", "/store/", s.Id())
	if err := cloudflare.Purge(s.Context(), url); err != nil {
		log.Error("Failed to purge store %v", err, s.Context())
	}
	return event.Emit(s.Context(), s.Namespace(), "store.updated", s)
}

func (s *Store) AfterDelete() error {
	url := config.UrlFor("api", "/store/", s.Id())
	if err := cloudflare.Purge(s.Context(), url); err != nil {
		log.Error("Failed to purge store %v", err, s.Context())
	}
	return event.Emit(s.Context(), s.Namespace(), "store.deleted", s)
}

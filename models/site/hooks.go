package site

import (
	"crowdstart.com/thirdparty/netlify"
	"crowdstart.com/util/webhook"
)

// Create
func (s *Site) BeforeCreate() error {
	nsite, err := netlify.CreateSite(s.Context(), s.Netlify())
	if err != nil {
		return err
	}

	s.SetNetlify(nsite)

	return nil
}

func (s *Site) AfterCreate() error {
	webhook.Emit(s.Context(), s.Namespace(), "site.created", s)
	return nil
}

// Update
func (s *Site) BeforeUpdate(previous *Site) error {
	nsite, err := netlify.UpdateSite(s.Context(), s.Netlify())
	if err != nil {
		return err
	}

	s.SetNetlify(nsite)

	return nil
}

func (s *Site) AfterUpdate(previous *Site) error {
	webhook.Emit(s.Context(), s.Namespace(), "site.spdated", s)
	return nil
}

// Delete
func (s *Site) BeforeDelete() error {
	if err := netlify.DeleteSite(s.Context(), s.Netlify()); err != nil {
		return err
	}
	return nil
}

func (s *Site) AfterDelete() error {
	webhook.Emit(s.Context(), s.Namespace(), "site.deleted", s)
	return nil
}

package site

import (
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/thirdparty/netlify"
)

type Site struct {
	mixin.BaseModel

	Domain string `json:"domain"`
	Name   string `json:"name"`
	Url    string `json:"url"`

	Netlify_ netlify.Site `json:"-"`
}

// Return netlify overriden with our local properties
func (s Site) Netlify() *netlify.Site {
	s.Netlify_.Name = s.Name
	s.Netlify_.Domain = s.Domain
	return &s.Netlify_
}

func (s *Site) SetNetlify(nsite *netlify.Site) {
	s.Netlify_ = *nsite
}

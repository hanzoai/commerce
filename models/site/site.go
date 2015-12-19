package site

import (
	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"
	"crowdstart.com/thirdparty/netlify"
)

type Site struct {
	mixin.Model

	Domain string `json:"domain"`
	Name   string `json:"name"`
	Url    string `json:"url"`

	Netlify_ netlify.Site `json:"-"`
}

func (s *Site) Init() {
}

func New(db *datastore.Datastore) *Site {
	s := new(Site)
	s.Init()
	s.Model = mixin.Model{Db: db, Entity: s}
	return s
}

func (s Site) Kind() string {
	return "site"
}

func (s Site) Document() mixin.Document {
	return &Document{
		Id_:    s.Id(),
		Name:   s.Name,
		Domain: s.Domain,
		Url:    s.Url,
	}
}

// Return netlify overriden with our local properties
func (s Site) Netlify() *netlify.Site {
	s.Netlify_.Name = s.Name
	s.Netlify_.CustomDomain = s.Domain
	return &s.Netlify_
}

func (s *Site) SetNetlify(nsite *netlify.Site) {
	s.Netlify_ = *nsite
}

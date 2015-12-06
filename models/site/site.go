package site

import (
	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"

	"github.com/netlify/netlify-go"
)

type Site struct {
	mixin.Model

	Domain string
	Name   string
	Url    string

	Netlify netlify.Site `json:"-"`
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
	return &Document{}
}

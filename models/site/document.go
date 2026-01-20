package site

import (
	"google.golang.org/appengine/search"

	"github.com/hanzoai/commerce/models/mixin"
)

type Document struct {
	// Special Kind Facet
	Kind search.Atom `search:",facet"`

	Id_    string
	Name   string
	Domain string
	Url    string
}

func (d Document) Id() string {
	return d.Id_
}

func (s Site) Document() mixin.Document {
	return &Document{
		search.Atom(kind),
		s.Id(),
		s.Name,
		s.Domain,
		s.Url,
	}
}

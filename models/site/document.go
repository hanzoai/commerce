package site

import (
	"appengine/search"

	"hanzo.io/models/mixin"
)

type Document struct {
	Kind search.Atom `search:",facet"`

	Id_    string
	Name   string
	Domain string
	Url    string
}

func (d Document) Id() string {
	return d.Id_
}

func (d Document) GetKind() string {
	return string(d.Kind)
}

func (d Document) SetKind(kind string) {
	d.Kind = search.Atom(kind)
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

package site

import (
	"appengine/search"

	"hanzo.io/models/mixin"
)

type Document struct {
	mixin.SearchKind

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
		mixin.SearchKind{
			search.Atom(kind),
		},
		s.Id(),
		s.Name,
		s.Domain,
		s.Url,
	}
}

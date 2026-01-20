package movie

import (
	"google.golang.org/appengine/search"

	"github.com/hanzoai/commerce/models/mixin"
)

type Document struct {
	mixin.DocumentSaveLoad `datastore:"-" json:"-"`

	// Special Kind Option
	Kind search.Atom `search:",facet"`

	Id_  string
	Slug string
	EIDR string
	IMDB string

	Name        string
	Headline    string
	Description string

	AvailableOption search.Atom `search:"available,facet"`
	HiddenOption    search.Atom `search:"hidden,facet"`
}

func (d *Document) Id() string {
	return d.Id_
}

func (d *Document) Init() {
	d.SetDocument(d)
}

func (m Movie) Document() mixin.Document {
	doc := &Document{}
	doc.Init()
	doc.Kind = search.Atom(kind)
	doc.Id_ = m.Id()
	doc.Slug = m.Slug
	doc.IMDB = m.IMDB
	doc.EIDR = m.EIDR

	if m.Available {
		doc.AvailableOption = "available"
	}

	if m.Hidden {
		doc.HiddenOption = "hidden"
	}

	return doc
}

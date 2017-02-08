package site

import (
	"hanzo.io/models/mixin"
)

type Document struct {
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
		Id_:    s.Id(),
		Name:   s.Name,
		Domain: s.Domain,
		Url:    s.Url,
	}
}

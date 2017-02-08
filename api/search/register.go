package search

import (
	"reflect"

	"hanzo.io/models/mixin"
)

var searches map[string]*Search

type Search struct {
	Kind    string
	Entity  mixin.Entity
	DocType reflect.Type
}

// Return a new document for underlying kind
func (s Search) Document() interface{} {
	return reflect.New(s.DocType).Interface()
}

// Register a new kind for search API
func Register(kind mixin.Kind, document mixin.Document) *Search {
	s := new(Search)

	s.Entity = mixin.EntityFromKind(kind)
	s.Kind = s.Entity.Kind()
	s.DocType = reflect.TypeOf(document)

	searches[s.Kind] = s

	return s
}

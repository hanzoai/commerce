package mixin

import (
	"google.golang.org/appengine/search"

	"hanzo.io/log"
)

var DefaultIndex = "everything"

type Document interface {
	Id() string
}

type Searchable interface {
	Document() Document
}

func (m Model) PutDocument() error {
	hook, ok := m.Entity.(Searchable)
	if !ok {
		// Not a searchable model, do nothing
		return nil
	}

	if doc := hook.Document(); doc != nil {
		index, err := search.Open(DefaultIndex)
		if err != nil {
			log.Error("Failed to open search index for model with id %v", m.Id(), m.Db.Context)
			return err
		}

		_, err = index.Put(m.Db.Context, m.Id(), doc)
		if err != nil {
			log.Error("Could not save search document for '%s' with id %s\nError: %s", m.Kind(), m.Id(), err, m.Db.Context)
			return err
		}
	}

	return nil
}

func (m Model) DeleteDocument() error {
	hook, ok := m.Entity.(Searchable)
	if !ok {
		// Not a searchable model, do nothing
		return nil
	}

	if doc := hook.Document(); doc != nil {
		index, err := search.Open(DefaultIndex)
		if err != nil {
			log.Error("Failed to open search index for model with id %v", m.Id(), m.Db.Context)
			return err
		}

		err = index.Delete(m.Db.Context, m.Id())
		if err != nil {
			log.Error("Could not delete search document for model with id %v", m.Id(), m.Db.Context)
			return err
		}
	}

	return nil
}

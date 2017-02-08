package mixin

import (
	"appengine/search"

	"hanzo.io/util/log"
)

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
		index, err := search.Open(m.Entity.Kind())
		if err != nil {
			log.Error("Failed to open search index for model with id %v", m.Id(), m.Db.Context)
			return err
		}

		_, err = index.Put(m.Db.Context, m.Id(), doc)
		if err != nil {
			log.Error("Could not save search document for model with id %v", m.Id(), m.Db.Context)
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
		index, err := search.Open(m.Entity.Kind())
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

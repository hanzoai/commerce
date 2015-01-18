package models

import (
	"net/http"
	"appengine/datastore"
	"github.com/mholt/binding"
)

// Flexible collection of datum
type Metadata map[string]Datum

func (m Metadata) Load(c <-chan datastore.Property) error {
	// Loop over properties stored and rebuild map
	for p := range c {
		m[p.Name] = p.Value.(Datum)
	}
	return nil
}

func (m Metadata) Save(c chan<- datastore.Property) error {
	defer close(c)
	// Loop over key, value pairs in instance and feed into datastore
	for k, v := range m {
		c <- datastore.Property {
			Name: k,
			Value: v,
		}
	}
	return nil
}

// A single piece of metadata
type Datum struct {
	Type  string
	Value string
}

func (m Datum) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	// Type and Value cannot be the empty string.

	if m.Type == "" {
		errs = append(errs, binding.Error{
			FieldNames:     []string{"Type"},
			Classification: "InputError",
			Message:        "Type cannot be empty.",
		})
	}

	if m.Value == "" {
		errs = append(errs, binding.Error{
			FieldNames:     []string{"Value"},
			Classification: "InputError",
			Message:        "Value cannot be empty.",
		})
	}
	return errs
}

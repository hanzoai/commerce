package mixin

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"sync"

	"google.golang.org/appengine/search"
)

///////// COPY PASTA /////////

// structCodec defines how to convert a given struct to/from a search document.
type structCodec struct {
	// byIndex returns the struct tag for the i'th struct field.
	byIndex []structTag

	// fieldByName returns the index of the struct field for the given field name.
	fieldByName map[string]int

	// facetByName returns the index of the struct field for the given facet name,
	facetByName map[string]int

	// I AM GREATDAVE
	ignoreByIndex map[int]bool
}

// structTag holds a structured version of each struct field's parsed tag.
type structTag struct {
	name  string
	facet bool
}

var (
	codecsMu    sync.RWMutex
	codecs      = map[reflect.Type]*structCodec{}
	fieldNameRE = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*$`)
)

// validFieldName is the Go equivalent of Python's _CheckFieldName. It checks
// the validity of both field and facet names.
func validFieldName(s string) bool {
	return len(s) <= 500 && fieldNameRE.MatchString(s)
}

func loadCodec(t reflect.Type) (*structCodec, error) {
	codecsMu.RLock()
	codec, ok := codecs[t]
	codecsMu.RUnlock()
	if ok {
		return codec, nil
	}

	codecsMu.Lock()
	defer codecsMu.Unlock()
	if codec, ok := codecs[t]; ok {
		return codec, nil
	}

	codec = &structCodec{
		fieldByName:   make(map[string]int),
		facetByName:   make(map[string]int),
		ignoreByIndex: make(map[int]bool),
	}

	for i, I := 0, t.NumField(); i < I; i++ {
		f := t.Field(i)
		name, opts := f.Tag.Get("search"), ""
		if i := strings.Index(name, ","); i != -1 {
			name, opts = name[:i], name[i+1:]
		}

		// TODO(davidday): Support name=="-" as per datastore.
		if name == "" {
			name = f.Name
		} else if name == "-" {
			codec.ignoreByIndex[i] = true
		} else if !validFieldName(name) {
			return nil, fmt.Errorf("search: struct tag has invalid field name: %q", name)
		}
		facet := opts == "facet"
		codec.byIndex = append(codec.byIndex, structTag{name: name, facet: facet})
		if facet {
			codec.facetByName[name] = i
		} else {
			codec.fieldByName[name] = i
		}
	}

	codecs[t] = codec
	return codec, nil
}

///////// END COPY PASTA /////////

type Document interface {
	Id() string
}

type DocumentLoadSaver struct {
	Document
}

func (d DocumentLoadSaver) Load(fields []search.Field, meta *search.DocumentMetadata) error {
	facets := meta.Facets
	for _, facet := range facets {
		fields = append(fields, search.Field{Name: facet.Name, Value: facet.Value})
	}
	return nil
}

func (d DocumentLoadSaver) Save() ([]search.Field, *search.DocumentMetadata, error) {
	v := reflect.ValueOf(d.Document).Elem()
	t := v.Type()

	codec, err := loadCodec(t)
	if err != nil {
		return nil, nil, err
	}

	fields := make([]search.Field, 0, len(codec.fieldByName))
	var facets []search.Facet
	for i, tag := range codec.byIndex {
		if codec.ignoreByIndex[i] {
			continue
		}

		f := v.Field(i)
		if !f.CanSet() {
			continue
		}

		if tag.facet {
			fval := f.Interface()
			switch val := fval.(type) {
			case string:
				facets = append(facets, search.Facet{Name: tag.name, Value: search.Atom(val)})
			case search.Atom:
				facets = append(facets, search.Facet{Name: tag.name, Value: val})
			case float64:
				facets = append(facets, search.Facet{Name: tag.name, Value: val})
			default:
				return nil, nil, errors.New("Unsupported Facet Type.")
			}
		} else {
			fields = append(fields, search.Field{Name: tag.name, Value: f.Interface()})
		}
	}

	return fields, &search.DocumentMetadata{Facets: facets}, nil
}

// Unified document type
type AnyDocument struct {
	Id_   string
	Kind_ string
}

func (d AnyDocument) Id() string {
	return string(d.Id_)
}

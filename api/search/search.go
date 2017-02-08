package search

import (
	"errors"
	"fmt"
	"strconv"

	aeds "google.golang.org/appengine/datastore"
	aesearch "google.golang.org/appengine/search"

	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	// "hanzo.io/models/order"

	"hanzo.io/models/mixin"
	"hanzo.io/util/hashid"
	"hanzo.io/util/json/http"
	"hanzo.io/util/log"
)

// type Facet
// type Response struct {
// 	Results []interface
// 	Facets  []Facet
// }

func parseQuery(c *gin.Context) (offset int, limit int, err error) {
	rawOffset := c.Query("offset")
	rawLimit := c.Query("limit")

	// Parse limit
	if rawLimit != "" {
		limit, err = strconv.Atoi(rawLimit)
		if err != nil {
			return offset, limit,
				errors.New(fmt.Sprintf("Unable to parse `limit` '%v': %v", limit, err))
		}

		if limit < 0 {
			return offset, limit,
				errors.New(fmt.Sprintf("`limit` must be an integer greater than 0: '%v'", limit))
		}
	}

	if rawOffset != "" {
		offset, err = strconv.Atoi(rawOffset)
		if err != nil {
			return offset, limit,
				errors.New(fmt.Sprintf("Unable to parse `offset` '%v': %v", offset, err))
		}

		// Ensure we have a offset
		if offset < 0 {
			return offset, limit,
				errors.New(fmt.Sprintf("`offset` must be an integer greater than 0: '%v'", offset))
		}

	}

	return offset, limit, nil
}

func searchAll(c *gin.Context) {
	// offset, limit, err := parseQuery(c)
	// if err != nil {
	// 	http.Fail(c, 404, "Unable to parse offset and limit", err)
	// 	return
	// }
	// q := c.Query("q")

	// index, err := aesearch.Open("all")
	// if err != nil {
	// 	http.Fail(c, 500, "Failed to find index 'all'", err)
	// 	return
	// }

	// db := datastore.New(middleware.GetNamespaced(c))

	// keys := make([]*aeds.Key, 0)
	// opts := &aesearch.SearchOptions{
	// 	Facets:  []aesearch.FacetSearchOption{aesearch.AutoFacetDiscovery(0, 0)},
	// 	IDsOnly: true,
	// 	Limit:   limit,
	// 	Offset:  offset,
	// }

	// for t := index.Search(db.Context, q, opts); ; {
	// 	doc := mixin.UnifiedDocument{}
	// 	_, err := t.Next(&doc) // We use the int id stored on the doc rather than the key
	// 	if err == aesearch.Done {
	// 		break
	// 	}
	// 	if err != nil {
	// 		http.Fail(c, 404, fmt.Sprintf("Failed to search index 'user' %v", err), err)
	// 		return
	// 	}

	// 	keys = append(keys, hashid.MustDecodeKey(db.Context, doc.Id()))
	// }

	// entities := make([]interface{}, 0)
	// if err := db.GetMulti(keys, &entities); err != nil {
	// 	http.Fail(c, 500, "Server side error", err)
	// 	return
	// }
	// http.Render(c, 200, entities)
}

func searchKind(c *gin.Context) {
	kind := c.Params.ByName("kind")

	// Accept query as `query` or `q`
	q := c.Query("query")
	if q == "" {
		q = c.Query("q")
	}

	s, ok := searches[kind]
	if !ok {
		http.Fail(c, 404, fmt.Sprintf("Invalid resource %v", kind), nil)
		return
	}

	offset, limit, err := parseQuery(c)
	if err != nil {
		http.Fail(c, 400, "Unable to parse offset and limit", err)
		return
	}

	index, err := aesearch.Open("all")
	if err != nil {
		http.Fail(c, 500, fmt.Sprintf("Failed to find index '%v'", kind), err)
		return
	}

	db := datastore.New(c)

	keys := make([]*aeds.Key, 0)
	opts := &aesearch.SearchOptions{
		// IDsOnly: true,
		Fields: []string{"Id_", "Kind_"},
		Limit:  limit,
		Offset: offset,
		Facets: []aesearch.FacetSearchOption{
			aesearch.AutoFacetDiscovery(0, 0),
		},
		Refinements: []aesearch.Facet{
			aesearch.Facet{Name: "Kind_", Value: aesearch.Atom(kind)},
		},
	}

	log.Debug("Searching index '%v' for: %v %#v", kind, q, opts)

	// Get Search iterator
	it := index.Search(db.Context, q, opts)

	// Get facets
	facets, err := it.Facets()
	log.Warn("Facets: %#v", facets)

	// Get Matching documents
	for {
		// Get a new fresh document for this search index
		doc := &mixin.AnyDocument{}

		key, err := it.Next(doc) // We use the int id stored on the doc rather than the key
		if err == aesearch.Done {
			break
		}

		if err != nil {
			http.Fail(c, 500, fmt.Sprintf("Failed to search index '%s': %v", kind, err), err)
			return
		}

		log.Warn("Found document: %#v", doc)

		keys = append(keys, hashid.MustDecodeKey(db.Context, key))
	}

	entities := s.Entity.ValueSlice(len(keys))

	if err := db.GetMulti(keys, entities); err != nil {
		http.Fail(c, 500, fmt.Sprintf("Failed to get %ss %v", s.Kind, err), err)
		return
	}

	http.Render(c, 200, entities)
}

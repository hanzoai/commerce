package search

import (
	"fmt"
	"strconv"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/datastore/iface"
	"github.com/hanzoai/commerce/datastore/key"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/models/order"
	"github.com/hanzoai/commerce/util/hashid"
	"github.com/hanzoai/commerce/util/json/http"
	"github.com/hanzoai/commerce/util/search"
)

func searchOrder(c *zip.Ctx) error {
	q := c.Query("q")

	opts := &search.SearchOptions{}
	limitStr := c.Query("limit")
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			opts.Limit = l
		}
	}

	o := order.Order{}
	index, err := search.Open(mixin.DefaultIndex)
	if err != nil {
		return http.Fail(c, 404, fmt.Sprintf("Failed to find index 'order'"), err)
	}

	db := datastore.New(middleware.GetNamespace(c))
	keys := make([]iface.Key, 0)
	for t := index.Search(db.Context, q, &search.SearchOptions{
		Refinements: []search.Facet{
			{
				Name:  "kind",
				Value: o.Kind(),
			},
		},
	}); ; {
		var doc order.Document
		_, err := t.Next(&doc) // We use the int id stored on the doc rather than the key
		if err == search.Done {
			break
		}
		if err != nil {
			return http.Fail(c, 404, fmt.Sprintf("Failed to search index 'order' %v", err), err)
		}

		keys = append(keys, key.FromDBKey(hashid.MustDecodeKey(db.Context, doc.Id())))
	}

	orders := make([]order.Order, len(keys))
	if err := db.GetMulti(keys, orders); err != nil {
		// http.Fail(c, 500, fmt.Sprintf("Failed to get orders %v", err), err)
		// return
	}

	return http.Render(c, 200, orders)
}

package search

import (
	"fmt"
	"strconv"

	aeds "appengine/datastore"
	"appengine/search"

	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/middleware"
	"hanzo.io/models/order"
	"hanzo.io/util/hashid"
	"hanzo.io/util/json/http"
)

func searchOrder(c *gin.Context) {
	q := c.Request.URL.Query().Get("q")

	opts := &search.SearchOptions{}
	limitStr := c.Request.URL.Query().Get("limit")
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			opts.Limit = l
		}
	}

	o := order.Order{}
	index, err := search.Open(o.Kind())
	if err != nil {
		http.Fail(c, 404, fmt.Sprintf("Failed to find index 'order'"), err)
		return
	}

	db := datastore.New(middleware.GetNamespace(c))

	keys := make([]*aeds.Key, 0)
	for t := index.Search(db.Context, q, nil); ; {
		var doc order.Document
		_, err := t.Next(&doc) // We use the int id stored on the doc rather than the key
		if err == search.Done {
			break
		}
		if err != nil {
			http.Fail(c, 404, fmt.Sprintf("Failed to search index 'order' %v", err), err)
			return
		}

		keys = append(keys, hashid.MustDecodeKey(db.Context, doc.Id()))
	}

	orders := make([]order.Order, len(keys))
	if err := db.GetMulti(keys, orders); err != nil {
		// http.Fail(c, 500, fmt.Sprintf("Failed to get orders %v", err), err)
		// return
	}

	http.Render(c, 200, orders)
}

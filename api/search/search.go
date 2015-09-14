package search

import (
	"fmt"
	"strconv"

	"appengine/search"

	"github.com/gin-gonic/gin"

	"crowdstart.com/datastore"
	"crowdstart.com/middleware"
	"crowdstart.com/models/order"
	"crowdstart.com/models/user"
	"crowdstart.com/util/json/http"
	"crowdstart.com/util/log"
)

func searchUser(c *gin.Context) {
	q := c.Request.URL.Query().Get("q")

	u := user.User{}
	index, err := search.Open(u.Kind())
	if err != nil {
		http.Fail(c, 404, fmt.Sprintf("Failed to find index 'user'"), err)
		return
	}

	db := datastore.New(middleware.GetNamespace(c))

	keys := make([]datastore.Key, 0)
	for t := index.Search(db.Context, q, nil); ; {
		var doc user.Document
		_, err := t.Next(&doc) // We use the int id stored on the doc rather than the key
		if err == search.Done {
			break
		}
		if err != nil {
			http.Fail(c, 404, fmt.Sprintf("Failed to search index 'user' %v", err), err)
			return
		}

		intId, err := strconv.Atoi(string(doc.IntId))
		if err != nil {
			http.Fail(c, 500, fmt.Sprintf("Failed to decode id for user %v", err), err)
			return
		}

		key := db.KeyFromInt(u.Kind(), int64(intId))
		keys = append(keys, key)
	}

	users := make([]user.User, len(keys))
	if err := db.GetMulti(keys, users); err != nil {
		http.Fail(c, 500, fmt.Sprintf("Failed to get users %v", err), err)
		return
	}

	http.Render(c, 200, users)
}

func searchOrder(c *gin.Context) {
	q := c.Request.URL.Query().Get("q")

	o := order.Order{}
	index, err := search.Open(o.Kind())
	if err != nil {
		http.Fail(c, 404, fmt.Sprintf("Failed to find index 'order'"), err)
		return
	}

	org := middleware.GetOrganization(c)
	db := datastore.New(middleware.GetNamespace(c))

	keys := make([]datastore.Key, 0)
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

		intId, err := strconv.Atoi(string(doc.IntId))
		if err != nil {
			http.Fail(c, 500, fmt.Sprintf("Failed to decode id for order %v", err), err)
			return
		}

		key := db.KeyFromInt(o.Kind(), int64(intId))
		log.Warn("IntId %v %v %v", org.Name, intId, key)

		keys = append(keys, key)
	}

	orders := make([]order.Order, len(keys))
	if err := db.GetMulti(keys, orders); err != nil {
		http.Fail(c, 500, fmt.Sprintf("Failed to get orders %v", err), err)
		return
	}

	log.Warn("wut %v", orders)

	http.Render(c, 200, orders)
}

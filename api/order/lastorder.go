package order

import (
	"errors"
	"strconv"

	"appengine"
	"appengine/datastore"
	"appengine/memcache"

	"github.com/gin-gonic/gin"

	"crowdstart.io/middleware"
	"crowdstart.io/models"
	"crowdstart.io/util/json"
	"crowdstart.io/util/log"
)

type Cached struct {
	Cursor string
	Order  *models.Order
}

// Get cached cursor/order
func getCached(ctx appengine.Context) (datastore.Cursor, *models.Order, error) {
	var cur datastore.Cursor
	ord := new(models.Order)
	cached := new(Cached)

	// Fetch cached items
	if item, err := memcache.Get(ctx, "cached_order"); err != nil {
		return cur, ord, errors.New("Unable to get cursor")
	} else {
		err := json.DecodeBytes(item.Value, cached)
		if err != nil {
			return cur, ord, err
		}
	}

	// Use cached order
	ord = cached.Order

	// Try to decode cursor
	cur, err := datastore.DecodeCursor(cached.Cursor)
	if err != nil {
		return cur, ord, errors.New("Unable to decode cursor")
	}

	return cur, ord, nil
}

// Update cached cursor/order
func setCached(ctx appengine.Context, cursor datastore.Cursor, ord *models.Order) error {
	return memcache.Set(ctx, &memcache.Item{
		Key:   "cached_order",
		Value: json.EncodeBytes(&Cached{cursor.String(), ord}),
	})
}

// Get most recent order from datastore
func getLastOrder(ctx appengine.Context) (datastore.Cursor, *models.Order, error) {
	var cur datastore.Cursor
	ord := new(models.Order)
	count, err := datastore.NewQuery("order").Count(ctx)
	if err != nil {
		return cur, ord, errors.New("Unable to get count for orders")
	}
	q := datastore.NewQuery("order").Order("CreatedAt").Offset(count - 1)
	it := q.Run(ctx)
	_, err = it.Next(ord)
	if err != nil {
		return cur, ord, errors.New("Unable to query next order")
	}
	cur, err = it.Cursor()
	return cur, ord, err
}

func LastOrder(c *gin.Context) {
	ctx := middleware.GetAppEngine(c)

	// Try to get cached cursor, order
	log.Debug("Get cached order")
	cur, ord, err := getCached(ctx)

	// First run
	if err != nil {
		log.Debug("Get last order the hard way")
		cur, ord, err = getLastOrder(ctx)
		if err != nil {
			c.Fail(500, errors.New("Failed to retrieve last order"))
		}
		setCached(ctx, cur, ord)
		c.String(200, strconv.FormatInt(ord.CreatedAt.Unix(), 10))
		return
	}

	// Subsequent runs, re-use cursor
	q := datastore.NewQuery("order").Start(cur)

	// Create iterator and return latest results
	neword := new(models.Order)
	it := q.Run(ctx)

	// Iterate over all orders until we hit the last
	log.Debug("Iterate over orders")

	for {
		log.Debug("Got next order")
		if _, err := it.Next(neword); err != nil {
			break
		}
	}

	// Update cache if we found a newer order
	if neword.CreatedAt.After(ord.CreatedAt) {
		log.Debug("Got newer order")
		ord = neword
		// Set cache
		setCached(ctx, cur, ord)
	}

	c.String(200, strconv.FormatInt(ord.CreatedAt.Unix(), 10))
}

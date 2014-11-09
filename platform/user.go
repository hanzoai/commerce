package admin

import (
	"appengine"
	"crowdstart.io/auth"
	"crowdstart.io/datastore"
	"crowdstart.io/models"
	"errors"
	"github.com/gin-gonic/gin"
)

func listOrders(c *gin.Context) {
	db := datastore.New(c)

	id, err := auth.GetUsername(c)

	if err != nil {
		return
	}

	var orders []models.Order
	q := db.Query("order").
		Filter("User.Id =", id)
	q.GetAll(db.Context, orders)

	// Render the template here
}

// Extracts the Order.Id from the url and removes it from the datastore.
func removeOrder(c *gin.Context) {
	d := c.Request.URL.Query().Get("orderId")
	ch := make(chan error)
	go db.RunInTransaction(func(c appengine.Context) (err error) {
		db := datastore.New(c)
		var orders [1]models.Order
		q := db.Query("order").
			Filter("Id =", id).
			Limit(1)

		keys, err := q.GetAll(c, orders)
		if err != nil {
			ch <- err
			return
		}

		if len(keys) < 1 {
			ch <- errors.New()
			return
		}

		orders[0].Cancelled = true
		_, err := db.Update(id, orders[0])
		ch <- err
	}, nil)

	// Return json success
}

package user

import (
	"errors"

	"crowdstart.io/auth"
	"crowdstart.io/datastore"
	"crowdstart.io/models"
	"github.com/gin-gonic/gin"

	"appengine"
)

func DisplayOrders(c *gin.Context) {
	user := auth.GetUser(c)
	if user == nil {
		log.Panic("User was not found")
	}

	orders := make([]interface{}, len(m.OrdersIds))
	for i, v := range orders {
		orders[i] = interface{}(v)
	}

	err = db.GetMulti(m.OrdersIds, orders)
	if err != nil {
		log.Panic("Error while retrieving orders", err)
	}

	o := make([]models.Order, len(orders))
	for i, v := range orders {
		o[i] = v.(models.Order)
	}

	// TODO: Filter shipped and pending orders
	// and pass into template.Render
	template.Render(c, "index.html", "orders", o)
}

func ModifyOrder(c *gin.Context) {

	// id := c.Request.URL.Query().Get("orderId")
}

// Extracts the Order.Id from the url and removes it from the datastore.
func RemoveOrder(c *gin.Context) {
	id := c.Request.URL.Query().Get("orderId")
	db := datastore.New(c)

	err := db.RunInTransaction(func(c appengine.Context) error {
		db := datastore.New(c)

		var order models.Order
		err := db.GetKey("order", id, order)
		if err != nil {
			return err
		}

		order.Cancelled = true
		_, err = db.Update(id, order)
		return err
	}, nil)

	if err == nil {
		//success
		return
	}
	// Return json of err
}

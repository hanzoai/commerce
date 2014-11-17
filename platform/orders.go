package platform

import (
	"appengine"
	"crowdstart.io/auth"
	"crowdstart.io/datastore"
	"crowdstart.io/models"
	"errors"
	"github.com/gin-gonic/gin"
)

// Gets the orders associated with a user id.
func Orders(c appengine.Context, id string) ([]models.Order, error) {
	db := datastore.New(c)

	var user models.User
	err := db.GetKey("user", id, user)
	if err != nil {
		return nil, err
	}

	var orders []models.Order
	err = db.GetKeyMulti("order", user.OrdersIds, orders)
	if err != nil {
		return nil, err
	}

	if orders == nil {
		return nil, errors.New("No orders found")
	}

	return orders, nil
}

func listOrders(c *gin.Context) {
	id, err := auth.GetUsername(c)
	if err != nil {
		return
	}
	orders := Orders(c, id)

	// TODO Figure out a way to separately display pending orders and completed orders.
	var pendingOrders []models.Order
	for order := range orders {
		if !order.Cancelled && !order.Shipped {
			append(pendingOrders, order)
		}
	}

	// Render the template using filtered orders
}

func modifyOrder(c *gin.Context) {
	id := c.Request.URL.Query().Get("orderId")
}

// Extracts the Order.Id from the url and removes it from the datastore.
func removeOrder(c *gin.Context) {
	id := c.Request.URL.Query().Get("orderId")
	db := datastore.New(c)

	err := db.RunInTransaction(func(c appengine.Context) error {
		db := datastore.New(c)

		var order models.Order
		err := db.GetKey("order", id, order)
		if err != nil {
			return err
		}

		if user == nil {
			return errors.New("User is nil")
		}

		order.Cancelled = true
		_, err = db.Update(id, orders)
		return err
	}, nil)

	if err == nil {
		//success
		return
	}
	// Return json of err
}

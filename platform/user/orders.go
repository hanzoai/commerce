package user

import (
	"appengine"
	"crowdstart.io/auth"
	"crowdstart.io/datastore"
	"crowdstart.io/models"
	"errors"
	"github.com/gin-gonic/gin"
)

// Gets the orders associated with a user id.
func Orders(ctx appengine.Context, id string) ([]models.Order, error) {
	db := datastore.New(ctx)

	var user models.User
	err := db.GetKey("user", id, user)
	if err != nil {
		return nil, err
	}

	rawOrders := make([]interface{}, len(user.OrdersIds))
	err = db.GetKeyMulti("order", user.OrdersIds, rawOrders)
	if err != nil {
		return nil, err
	}

	if rawOrders == nil {
		return nil, errors.New("No orders found")
	}

	orders := make([]models.Order, len(rawOrders))

	for i := range orders {
		orders[i] = rawOrders[i].(models.Order)
	}

	return orders, nil
}

func ListOrders(c *gin.Context) {
	id, err := auth.GetUsername(c)
	if err != nil {
		return
	}
	ctx := c.MustGet("appengine").(appengine.Context)
	orders, err := Orders(ctx, id)

	if err != nil {

	}

	// TODO Figure out a way to separately display pending orders and completed orders.
	var pendingOrders []models.Order
	for _, order := range orders {
		if !order.Cancelled && !order.Shipped {
			pendingOrders = append(pendingOrders, order)
		}
	}

	// Render the template using filtered orders
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

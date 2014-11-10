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

	var user models.User
	err = db.GetKey("user", id, user)
	if err != nil {
		return
	}
	if user == nil {
		return
	}

	var orders []models.Order
	err = db.GetKeyMulti("order", user.OrdersIds, orders)
	if err != nil {
		return
	}
	if orders == nil {
		return
	}

	// Render the template using orders here
}

func modifyOrder(c *gin.Context) {
	id := c.Request.URL.Query().Get("orderId")
}

// Extracts the Order.Id from the url and removes it from the datastore.
func removeOrder(c *gin.Context) {
	id := c.Request.URL.Query().Get("orderId")
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

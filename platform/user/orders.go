package user

import (
	"crowdstart.io/datastore"
	"crowdstart.io/models"
	"github.com/gin-gonic/gin"

	"appengine"
)

func DisplayOrders(c *gin.Context) {
}

func ModifyOrder(c *gin.Context) {
	f := new(OrderForm)
	f.Parse(c)
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

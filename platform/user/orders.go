package user

import (
	"errors"

	"github.com/gin-gonic/gin"

	"appengine"

	"crowdstart.io/auth"
	"crowdstart.io/datastore"
	"crowdstart.io/models"
	"crowdstart.io/util/form"
)

func DisplayOrders(c *gin.Context) {

}

func ModifyOrder(c *gin.Context) {
	o := new(models.Order)
	err := form.Parse(c, o)
	if err != nil {
		c.Fail(500, err)
		return
	}

	db := datastore.New(c)
	_, err = db.Update(o.Id, o)
	if err != nil {
		c.Fail(500, err)
		return
	}

	c.JSON(200, o)
}

// Extracts the orderId from the url and removes it from the datastore.
func RemoveOrder(c *gin.Context) {
	id := c.Request.URL.Query().Get("orderId")
	user := auth.GetUser(c)

	hasOrder := false
	for _, _id := range user.OrdersIds {
		if _id == id {
			hasOrder = true
			break
		}
	}
	if !hasOrder {
		c.Fail(500, errors.New("Invalid order id"))
	}

	db := datastore.New(c)

	err := db.RunInTransaction(func(c appengine.Context) error {
		db := datastore.New(c)

		var order models.Order
		err := db.GetKey("order", id, order)
		if err != nil {
			return err
		}

		order.Cancelled = true
		_, err = db.PutKey("order", id, order)
		return err
	}, nil)

	if err == nil {
		//success
		return
	}
	// Return json of err
}

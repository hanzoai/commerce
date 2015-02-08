package user

import (
	"errors"

	"github.com/gin-gonic/gin"

	"appengine"

	// "crowdstart.io/auth"
	"crowdstart.io/datastore"
	"crowdstart.io/models"
	"crowdstart.io/util/form"
	"crowdstart.io/util/json"
	"crowdstart.io/util/template"
)

func DisplayOrders(c *gin.Context) {
	// user := auth.GetUser(c)
	// db := datastore.New(c)
	var orders []interface{}
	// err := db.GetKindMulti("order", user.OrdersIds, orders)
	// if err != nil {
	// 	c.Fail(500, err)
	// 	return
	// }

	var cancelled []models.Order
	var shipped []models.Order
	var pending []models.Order

	for _, o := range orders {
		order := o.(models.Order)
		switch {
		case order.Shipped:
			shipped = append(shipped, order)

		case order.Cancelled:
			cancelled = append(cancelled, order)

		default:
			pending = append(pending, order)
		}
	}

	template.Render(c, "platform/user/orders.html",
		"cancelled", json.Encode(cancelled),
		"shipped", json.Encode(shipped),
		"pending", json.Encode(pending),
	)
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
	// user := auth.GetUser(c)

	hasOrder := false
	// for _, _id := range user.OrdersIds {
	// 	if _id == id {
	// 		hasOrder = true
	// 		break
	// 	}
	// }
	if !hasOrder {
		c.Fail(500, errors.New("Invalid order id"))
	}

	db := datastore.New(c)

	err := db.RunInTransaction(func(c appengine.Context) error {
		db := datastore.New(c)

		var order models.Order
		err := db.GetKind("order", id, order)
		if err != nil {
			return err
		}

		order.Cancelled = true
		_, err = db.PutKind("order", id, order)
		return err
	}, nil)

	if err == nil {
		//success
		return
	}
	// Return json of err
}

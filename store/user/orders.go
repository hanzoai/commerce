package user

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/auth"
	"crowdstart.io/datastore"
	"crowdstart.io/models"
	"crowdstart.io/util/log"
	"crowdstart.io/util/template"
)

func DisplayOrders(c *gin.Context) {
	user := auth.GetUser(c)
	db := datastore.New(c)

	var genOrders []interface{}
	err := db.GetKeyMulti("order", user.OrdersIds, genOrders)
	if err != nil {
		c.Fail(500, err)
		return
	}

	orders := make([]models.Order, len(genOrders))
	for i, order := range genOrders {
		orders[i] = order.(models.Order)
	}

	// SKULLY Preorder
	// Searches for an order where the user's email is the key
	preorder := new(models.Order)
	err = db.GetKey("order", user.Email, preorder)
	if err == nil {
		preorder.Preorder = true
		orders = append(orders, *preorder)
	} else {
		log.Debug("User doesn't have a preorder. %s", user.Email)
	}

	template.Render(c, "orders.html",
		"orders", orders,
	)
}

type CancelOrderStatus struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func CancelOrder(c *gin.Context) {
	orderId := c.Request.URL.Query().Get("id")
	user := auth.GetUser(c)

	exists := user.Email == orderId // For SKULLY preorder
	if !exists {
		for _, id := range user.OrdersIds {
			if id == orderId {
				exists = true
				break
			}
		}
	}

	if !exists {
		log.Panic("Invalid order id")
	}

	db := datastore.New(c)

	order := new(models.Order)
	err := db.GetKey("user", orderId, order)
	if err != nil {
		log.Panic("Error while retrieving order \n%v", err)
	}

	if order.Shipped {
		c.JSON(200, CancelOrderStatus{false, "The order has already been shipped."})
		return
	}

	if order.Cancelled {
		c.JSON(200, CancelOrderStatus{false, "The order has already been cancelled."})
		return
	}

	order.Cancelled = true
	_, err = db.PutKey("user", orderId, order)
	if err != nil {
		c.JSON(500, CancelOrderStatus{false, "Error occurred while cancelling."})
		log.Panic("Erroring while saving order \n%v", err)
	}

	c.JSON(200, CancelOrderStatus{true, "The order is cancelled."})
}

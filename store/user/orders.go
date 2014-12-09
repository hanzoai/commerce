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
	email, err := auth.GetEmail(c)
	if err == nil {
		log.Panic("Error getting logged in user from the datastore \n%v", err)
	}

	db := datastore.New(c)
	var genOrders []interface{}
	_, err = db.Query("order").
		Filter("Email =", email).
		GetAll(db.Context, genOrders)

	if err != nil {
		log.Panic("Error retrieving orders associated with the user's email", err)
	}

	orders := make([]models.Order, len(genOrders))
	for i, o := range genOrders {
		orders[i] = o.(models.Order)
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

	email, err := auth.GetEmail(c)
	if err != nil {
		log.Panic("Error retrieving user \n%v", err)
	}

	db := datastore.New(c)

	order := new(models.Order)
	if err := db.GetKey("order", orderId, order); err != nil {
		log.Panic("Error while retrieving order \n%v", err)
	}

	if order.Email != email {
		log.Panic("Email associated with order does not match the email retrieved from the session \nSessionEmail: %s \n%#v", email, order)
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

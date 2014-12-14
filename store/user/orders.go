package user

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"crowdstart.io/auth"
	"crowdstart.io/datastore"
	"crowdstart.io/models"
	"crowdstart.io/util/log"
	"crowdstart.io/util/template"
)

// GET store./orders
// LoginRequired
func ListOrders(c *gin.Context) {
	email, err := auth.GetEmail(c)
	if err != nil {
		log.Panic("Error getting email from the session \n%v", err)
	}

	db := datastore.New(c)

	var orders []models.Order
	keys, err := db.Query("order").
		Filter("Email =", email).
		GetAll(db.Context, &orders)

	if err != nil {
		log.Panic("Error retrieving orders associated with the user's email", err)
	}

	for i := range orders {
		orders[i].LoadVariantsProducts(c)
		orders[i].Id = strconv.Itoa(int(keys[i].IntID()))
	}

	var tokens []models.Token
	_, err = db.Query("invite-token").
		Filter("Email =", email).
		Limit(1).
		GetAll(db.Context, &tokens)

	tokenId := ""
	if len(tokens) > 0 {
		tokenId = tokens[0].Id
	}

	template.Render(c, "orders.html",
		"orders", orders,
		"tokenId", tokenId,
	)
}

type CancelOrderStatus struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// Do not route to this.
// May be useful in the future
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

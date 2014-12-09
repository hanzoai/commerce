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
	// err := db.GetKeyMulti("order", user.OrdersIds, genOrders)
	// if err != nil {
	// 	c.Fail(500, err)
	// 	return
	// }

	orders := make([]models.Order, len(genOrders))
	for i, order := range genOrders {
		orders[i] = order.(models.Order)
	}

	// SKULLY Preorder
	// Searches for an order where the user's email is the key
	preorder := new(models.Order)
	err := db.GetKey("order", user.Email, preorder)
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

package admin

import (
	"github.com/gin-gonic/gin"
	"crowdstart.io/datastore"
	"crowdstart.io/models"
	"crowdstart.io/auth"
)

func listOrders(c *gin.Context) {
	db := datastore.New(c)
	id, err := auth.GetUsername(c)

	if err != nil {
		return
	}
	
	var orders []models.Order
	q := db.Query("order").
		Filter("User.Id =", id)
	q.GetAll(db.Context, orders)

	// Render the template here
}

// Extracts the Order.Id from the url and removes it from the datastore.
func removeOrder(c *gin.Context) {
	id := c.Request.URL.Query().Get("orderId")
	db := datastore.New(c)
	db.Delete(id)

	// Return json success
}

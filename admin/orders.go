package admin

import (
//	"appengine"
	"crowdstart.io/auth"
	"crowdstart.io/datastore"
	"crowdstart.io/models"
	"github.com/gin-gonic/gin"
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
	
	/*id := c.Request.URL.Query().Get("orderId")
	
	db.RunInTransaction(func(c appengine.Context) (err error) {
		db := datastore.New(c)
	})*/

	// Return json success
}

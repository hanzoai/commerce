package admin

import (
	"github.com/gin-gonic/gin"
	"crowdstart.io/datastore"
	"crowdstart.io/models"
	"crowdstart.io/auth"
)

func orders(c *gin.Context) {
	db := datastore.New(c)
	id, err := auth.GetId(c)

	if err != nil {
		return
	}
	
	var orders []models.Order
	q := db.Query("order").
		Filter("User.Id =", id)
	q.GetAll(db.Context, orders)
}

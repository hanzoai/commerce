package order

import (
	"github.com/gin-gonic/gin"
	// "hanzo.io/datastore"
	// "hanzo.io/middleware"
	// "hanzo.io/models/order"
	// "hanzo.io/util/json"
	// "hanzo.io/util/json/http"
)

type StatusResponse struct {
}

func Status(c *gin.Context) {
	// org := middleware.GetOrganization(c)
	// db := datastore.New(org.Namespaced(c))
	// ord := order.New(db)

	// // Ensure order exists
	// if err := ord.GetById(id); err != nil {
	// 	http.Fail(c, 404, "No order found with id: "+id, err)
	// 	return
	// }
}

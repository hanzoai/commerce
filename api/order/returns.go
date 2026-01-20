package order

import (
	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/datastore"
	return_ "github.com/hanzoai/commerce/models/return"
	"github.com/hanzoai/commerce/util/json/http"
)

func Returns(c *gin.Context) {
	id := c.Params.ByName("orderid")
	db := datastore.New(c)

	rtns := make([]*return_.Return, 0)
	return_.Query(db).Filter("OrderId=", id).GetAll(&rtns)
	http.Render(c, 200, rtns)
}

package order

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	return_ "hanzo.io/models/return"
	"hanzo.io/util/json/http"
)

func Returns(c *context.Context) {
	id := c.Params.ByName("orderid")
	db := datastore.New(c)

	rtns := make([]*return_.Return, 0)
	return_.Query(db).Filter("OrderId=", id).GetAll(&rtns)
	http.Render(c, 200, rtns)
}

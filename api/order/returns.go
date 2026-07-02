package order

import (
	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/middleware"
	return_ "github.com/hanzoai/commerce/models/return"
	"github.com/hanzoai/commerce/util/json/http"
)

func Returns(c *gin.Context) {
	id := c.Params.ByName("orderid")
	// Per-org store (Red MED-1): returns are children of the order in the caller
	// org's store; systemDB (datastore.New) would list nothing once the resolver
	// is installed.
	org := middleware.GetOrganization(c)
	db := datastore.NewNamespaced(org.Namespaced(c))

	rtns := make([]*return_.Return, 0)
	return_.Query(db).Filter("OrderId=", id).GetAll(&rtns)
	http.Render(c, 200, rtns)
}

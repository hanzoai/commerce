package order

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/middleware"
	return_ "github.com/hanzoai/commerce/models/return"
	"github.com/hanzoai/commerce/util/json/http"
)

func Returns(c *zip.Ctx) error {
	id := c.Param("orderid")
	// Per-org store (Red MED-1): returns are children of the order in the caller
	// org's store; systemDB (datastore.New) would list nothing once the resolver
	// is installed.
	org := middleware.GetOrganization(c)
	db := datastore.NewNamespaced(org.Namespaced(c.Context()))

	rtns := make([]*return_.Return, 0)
	return_.Query(db).Filter("OrderId=", id).GetAll(&rtns)
	return http.Render(c, 200, rtns)
}

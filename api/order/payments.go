package order

import (
	"fmt"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/order"
	"github.com/hanzoai/commerce/models/payment"
	"github.com/hanzoai/commerce/util/json/http"
)

func Payments(c *zip.Ctx) error {
	id := c.Param("orderid")
	// Per-org store (Red MED-1): the order + its payments live in the caller
	// org's store. datastore.New(c) drops the namespace (gin.Context → Background)
	// and binds systemDB, so it 404s the order once the resolver is installed.
	org := middleware.GetOrganization(c)
	db := datastore.NewNamespaced(org.Namespaced(c.Context()))
	ord := order.New(db)

	err := ord.GetById(id)
	if err != nil {
		return http.Fail(c, 404, fmt.Sprintf("Failed to retrieve order %v: %v", id, err), err)
	}

	payments := make([]*payment.Payment, 0)
	payment.Query(db).Ancestor(ord.Key()).GetAll(&payments)
	return http.Render(c, 200, payments)
}

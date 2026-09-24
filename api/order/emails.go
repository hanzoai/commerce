package order

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/email"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/order"
	"github.com/hanzoai/commerce/models/payment"
	"github.com/hanzoai/commerce/models/user"
)

func SendOrderConfirmation(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	// Per-org store (Red MED-1): the order is read via MustGetById, which PANICS
	// on a miss — datastore.New binds systemDB, so once the resolver is installed
	// the per-org order isn't found and the receipt handler 500s/panics.
	db := datastore.NewNamespaced(org.Namespaced(c.Context()))

	o := order.New(db)
	id := c.Param("orderid")
	o.MustGetById(id)

	u := user.New(db)
	u.MustGetById(o.UserId)

	email.SendOrderConfirmation(db.Context, org, o, u)

	return c.NoContent(204)
}

func SendFulfillmentConfirmation(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	// Per-org store (Red MED-1): the order is read via MustGetById, which PANICS
	// on a miss — datastore.New binds systemDB, so once the resolver is installed
	// the per-org order isn't found and the receipt handler 500s/panics.
	db := datastore.NewNamespaced(org.Namespaced(c.Context()))

	o := order.New(db)
	id := c.Param("orderid")
	o.MustGetById(id)

	u := user.New(db)
	u.MustGetById(o.UserId)

	p := payment.New(db)
	p.MustGetById(o.PaymentIds[0])

	email.SendOrderShipped(db.Context, org, o, u, p)

	return c.NoContent(204)
}

func SendRefundConfirmation(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	// Per-org store (Red MED-1): the order is read via MustGetById, which PANICS
	// on a miss — datastore.New binds systemDB, so once the resolver is installed
	// the per-org order isn't found and the receipt handler 500s/panics.
	db := datastore.NewNamespaced(org.Namespaced(c.Context()))

	o := order.New(db)
	id := c.Param("orderid")
	o.MustGetById(id)

	u := user.New(db)
	u.MustGetById(o.UserId)

	p := payment.New(db)
	p.MustGetById(o.PaymentIds[0])

	if o.Refunded == o.Paid {
		email.SendOrderRefunded(db.Context, org, o, u, p)
	} else if o.Refunded > 0 {
		email.SendOrderPartiallyRefunded(db.Context, org, o, u, p)
	}

	return c.NoContent(204)
}

package order

import (
	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/order"
	"github.com/hanzoai/commerce/models/payment"
	"github.com/hanzoai/commerce/models/user"
	"github.com/hanzoai/commerce/email"
)

func SendOrderConfirmation(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(middleware.GetNamespace(c))

	o := order.New(db)
	id := c.Params.ByName("orderid")
	o.MustGetById(id)

	u := user.New(db)
	u.MustGetById(o.UserId)

	email.SendOrderConfirmation(db.Context, org, o, u)

	c.Writer.WriteHeader(204)
}

func SendFulfillmentConfirmation(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(middleware.GetNamespace(c))

	o := order.New(db)
	id := c.Params.ByName("orderid")
	o.MustGetById(id)

	u := user.New(db)
	u.MustGetById(o.UserId)

	p := payment.New(db)
	p.MustGetById(o.PaymentIds[0])

	email.SendOrderShipped(db.Context, org, o, u, p)

	c.Writer.WriteHeader(204)
}

func SendRefundConfirmation(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(middleware.GetNamespace(c))

	o := order.New(db)
	id := c.Params.ByName("orderid")
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

	c.Writer.WriteHeader(204)
}


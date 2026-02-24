package tasks

import (
	"context"

	"github.com/hanzoai/commerce/api/checkout/util"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/delay"
	"github.com/hanzoai/commerce/email"
	"github.com/hanzoai/commerce/models/order"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/user"
)

var CaptureAsync = delay.Func("capture-async", func(ctx context.Context, orgId string, ordId string) {
	db := datastore.New(ctx)
	org := organization.New(db)
	org.MustGetById(orgId)

	nsdb := datastore.New(org.Namespaced(ctx))
	ord := order.New(nsdb)
	usr := user.New(nsdb)

	ord.MustGetById(ordId)
	usr.MustGetById(ord.UserId)

	if !ord.Test || usr.Test {
		util.UpdateMailchimp(ctx, org, ord, usr)
	}

	// payments := make([]*payment.Payment, 0)
	// if _, err := payment.Query(nsdb).Ancestor(ord.Key()).GetAll(payments); err != nil {
	// 	log.Error("Unable to find payments associated with order '%s'", ord.Id())
	// }

	// sendOrderConfirmation(ctx, org, ord, payments[0].Buyer)
	// saveRedemptions(ctx, ord)
	// saveReferral(ctx, org, ord)
	// updateCart(ctx, ord)
	// updateStats(ctx, org, ord, payments)
	// updateMailchimp(ctx, org, ord)
})

var SendOrderConfirmation = delay.Func("send-order-confirmation", func(ctx context.Context, orgId, ordId, emailAddress, firstName, lastName string) {
	db := datastore.New(ctx)
	org := organization.New(db)
	org.MustGetById(orgId)

	nsdb := datastore.New(org.Namespaced(ctx))
	ord := order.New(nsdb)
	ord.MustGetById(ordId)

	// Send Create user
	usr := new(user.User)
	usr.Email = emailAddress
	usr.FirstName = firstName
	usr.LastName = lastName
	usr.Init(ord.Datastore())

	email.SendOrderConfirmation(ctx, org, ord, usr)
})

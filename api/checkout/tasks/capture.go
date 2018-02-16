package tasks

import (
	"google.golang.org/appengine"

	"hanzo.io/api/checkout/util"
	"hanzo.io/datastore"
	"hanzo.io/models/order"
	"hanzo.io/models/organization"
	"hanzo.io/models/user"
	"hanzo.io/util/delay"
	"hanzo.io/util/emails"
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

	util.UpdateMailchimp(ctx, org, ord, usr)

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

var SendOrderConfirmation = delay.Func("send-order-confirmation", func(ctx context.Context, orgId, ordId, email, firstName, lastName string) {
	db := datastore.New(ctx)
	org := organization.New(db)
	org.MustGetById(orgId)

	nsdb := datastore.New(org.Namespaced(ctx))
	ord := order.New(nsdb)
	ord.MustGetById(ordId)

	// Send Create user
	usr := new(user.User)
	usr.Email = email
	usr.FirstName = firstName
	usr.LastName = lastName
	usr.Db = ord.Db
	usr.Entity = usr

	emails.SendOrderConfirmationEmail(ctx, org, ord, usr)
})

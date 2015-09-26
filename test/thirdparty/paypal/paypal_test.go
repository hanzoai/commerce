package test

import (
	"github.com/zeekay/aetest"

	"crowdstart.com/datastore"
	"crowdstart.com/models/order"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/payment"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/models/user"

	. "crowdstart.com/util/test/ginkgo"
)

var (
	ctx aetest.Context
	org *organization.Organization
	usr *user.User
	ord *order.Order
	pay *payment.Payment
)

var _ = BeforeSuite(func() {
	var err error
	ctx, err = aetest.NewContext(&aetest.Options{StronglyConsistentDatastore: true})
	Expect(err).ToNot(HaveOccurred())

	db := datastore.New(ctx)

	usr = user.New(db)
	usr.PaypalEmail = ""

	org = organization.New(db)
	org.Paypal.Email = ""
	org.Paypal.ConfirmUrl = ""
	org.Paypal.CancelUrl = ""
	org.Fee = 0.05

	pay = payment.New(db)
	pay.Amount = 100
	pay.Currency = currency.USD
	pay.Client.Ip = "1.1.1.1"
})

var _ = AfterSuite(func() {
	err := ctx.Close()
	Expect(err).ToNot(HaveOccurred())
})

var _ = Describe("paypal.GetPayKey", func() {
	Context("Should Succeed", func() {
		// Do things here
	})
})

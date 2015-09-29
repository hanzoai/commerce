package test

import (
	"testing"

	"github.com/zeekay/aetest"

	"crowdstart.com/datastore"
	"crowdstart.com/models/order"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/payment"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/models/user"
	"crowdstart.com/thirdparty/paypal"

	. "crowdstart.com/util/test/ginkgo"
)

func Test(t *testing.T) {
	Setup("thirdparty/paypal", t)
}

var (
	ctx    aetest.Context
	org    *organization.Organization
	usr    *user.User
	ord    *order.Order
	pay    *payment.Payment
	client *paypal.Client
)

var _ = BeforeSuite(func() {
	var err error
	ctx, err = aetest.NewContext(&aetest.Options{StronglyConsistentDatastore: true})
	Expect(err).ToNot(HaveOccurred())

	db := datastore.New(ctx)

	usr = user.New(db)
	usr.PaypalEmail = "brokeima@gmail.com"

	org = organization.New(db)
	org.Paypal.Email = "paypal@suchtees.com"
	org.Paypal.ConfirmUrl = "localhost:8080"
	org.Paypal.CancelUrl = "localhost:8080"
	org.Fee = 0.05

	pay = payment.New(db)
	pay.Amount = 100
	pay.Currency = currency.USD
	pay.Client.Ip = "1.1.1.1"

	client = paypal.New(ctx)
})

var _ = AfterSuite(func() {
	err := ctx.Close()
	Expect(err).ToNot(HaveOccurred())
})

var _ = Describe("paypal.GetPayKey", func() {
	Context("Get Paypal PayKey", func() {
		It("Should succeed in the normal case", func() {
			key, err := client.GetPayKey(pay, usr, org)
			Expect(err).ToNot(HaveOccurred())
			Expect(key).ToNot(Equal(""))
		})
	})
})

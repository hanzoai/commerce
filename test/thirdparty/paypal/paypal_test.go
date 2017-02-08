package test

import (
	"testing"

	"golang.org/x/net/context"

	"hanzo.io/datastore"
	"hanzo.io/models/order"
	"hanzo.io/models/organization"
	"hanzo.io/models/payment"
	"hanzo.io/models/types/currency"
	"hanzo.io/models/user"
	"hanzo.io/thirdparty/paypal"
	"hanzo.io/util/test/ae"

	. "hanzo.io/models/lineitem"
	. "hanzo.io/util/test/ginkgo"
)

func Test(t *testing.T) {
	Setup("thirdparty/paypal", t)
}

var (
	ctx    context.Context
	inst   ae.Instance
	org    *organization.Organization
	usr    *user.User
	ord    *order.Order
	pay    *payment.Payment
	client *paypal.Client
)

var _ = BeforeSuite(func() {
	var err error
	ctx, inst, err = ae.NewContext()
	Expect(err).ToNot(HaveOccurred())

	db := datastore.New(ctx)

	usr = user.New(db)
	// usr.PaypalEmail = "dev@hanzo.ai"

	org = organization.New(db)
	org.Paypal.ConfirmUrl = "http://www.hanzo.io"
	org.Paypal.CancelUrl = "http://www.hanzo.io"

	org.Paypal.Test.Email = "dev@hanzo.ai"
	org.Paypal.Test.SecurityUserId = "dev@hanzo.ai"
	org.Paypal.Test.ApplicationId = "APP-80W284485P519543T"
	org.Paypal.Test.SecurityPassword = ""
	org.Paypal.Test.SecuritySignature = ""

	ord = order.New(db)
	ord.Items = make([]LineItem, 1)
	ord.Items[0] = LineItem{
		ProductId:   "Test Product Id",
		ProductName: "Test Product Name",
		ProductSlug: "Test Product Slug",
		Price:       100,
		Quantity:    1,
	}
	ord.Currency = currency.USD
	ord.Tax = 1
	ord.Shipping = 2
	ord.Total = 103

	pay = payment.New(db)
	pay.Amount = 103
	pay.Currency = currency.USD
	pay.Client.Ip = "64.136.209.186"
	pay.Fee = ord.CalculateFee(org.Fee)

	client = paypal.New(ctx)
})

var _ = AfterSuite(func() {
	err := inst.Close()
	Expect(err).ToNot(HaveOccurred())
})

var _ = Describe("paypal.GetPayKey", func() {
	Context("Get Paypal PayKey", func() {
		It("Should succeed in the normal case", func() {
			key, err := client.GetPayKey(pay, usr, ord, org)
			Expect(err).ToNot(HaveOccurred())
			Expect(key).ToNot(Equal(""))
		})
	})
})

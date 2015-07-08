package test

import (
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"crowdstart.com/datastore"
	"crowdstart.com/models/order"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/payment"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/util/gincontext"
	"crowdstart.com/util/test/ae"
	"crowdstart.com/util/test/ginclient"

	. "crowdstart.com/test/thirdparty/stripe/request"
	_ "crowdstart.com/thirdparty/stripe/tasks"
	stripeApi "crowdstart.com/thirdparty/stripe/webhook"
	. "crowdstart.com/util/test/ginkgo"
)

var (
	c      *gin.Context
	ctx    ae.Context
	client *ginclient.Client
	db     *datastore.Datastore
	org    *organization.Organization
)

func Test(t *testing.T) {
	Setup("thirdparty/stripe/webhook", t)
}

var _ = BeforeSuite(func() {
	ctx = ae.NewContext(ae.Options{
		Modules:    []string{"default"},
		TaskQueues: []string{"default"},
		Noisy:      true,
	})
	c = gincontext.New(ctx)
	db = datastore.New(c)

	org = organization.New(db)
	org.Stripe.UserId = "1"
	org.Stripe.Test.UserId = "1"
	org.Put()

	client = ginclient.New(ctx)
	client.Setup(func(r *http.Request) {})

	stripeApi.Route(client.Router)
})

var _ = AfterSuite(func() {
	ctx.Close()
})

func mockStripeEvent(event, status string, captured bool) (*order.Order, *payment.Payment) {
	refunded := false

	ord := order.New(db)
	ord.Put()

	pay := payment.New(db)
	pay.OrderId = ord.Id()
	pay.Amount = currency.Cents(1000)
	if status == "refunded" {
		pay.AmountRefunded = pay.Amount
		refunded = true
	}
	pay.Put()

	ord.PaymentIds = []string{pay.Id()}
	ord.Total = currency.Cents(1000)
	ord.Put()

	request := CreateRequest(event, ord.Id(), pay.Id(), status, refunded, captured)
	w := client.PostRawJSON("/stripe/webhook", request)
	Expect(w.Code).To(Equal(200))

	time.Sleep(10 * time.Second)

	pay2 := payment.New(db)
	pay2.GetById(pay.Id())

	ord2 := order.New(db)
	ord2.GetById(ord.Id())

	return ord2, pay2
}

var _ = Describe("Stripe Webhook Events", func() {
	Context("Respond To charge.updated Events", func() {
		It("Succeeded = true", func() {
			ord, pay := mockStripeEvent("charge.updated", "succeeded", true)

			Expect(payment.Paid).To(Equal(string(pay.Status)))
			Expect(ord.Paid).To(Equal(pay.Amount))
			Expect(order.Open).To(Equal(ord.Status))
		})

		It("Status = failed", func() {
			ord, pay := mockStripeEvent("charge.updated", "failed", true)

			Expect(payment.Cancelled).To(Equal(pay.Status))
			Expect(payment.Cancelled).To(Equal(ord.PaymentStatus))
			Expect(order.Cancelled).To(Equal(string(ord.Status)))
		})

		It("Status = refunded", func() {
			ord, pay := mockStripeEvent("charge.updated", "refunded", true)

			Expect(payment.Refunded).To(Equal(string(pay.Status)))
			Expect(payment.Refunded).To(Equal(string(ord.PaymentStatus)))
			Expect(order.Cancelled).To(Equal(string(ord.Status)))
		})

		It("Status = disputed", func() {
			ord, pay := mockStripeEvent("charge.updated", "disputed", true)

			Expect(payment.Refunded).To(Equal(string(pay.Status)))
			Expect(payment.Refunded).To(Equal(string(ord.PaymentStatus)))
			Expect(order.Disputed).To(Equal(string(ord.Status)))
		})
	})
})

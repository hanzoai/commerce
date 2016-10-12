package test

import (
	"errors"
	"net/http"
	"testing"

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
	c   *gin.Context
	ctx ae.Context
	cl  *ginclient.Client
	db  *datastore.Datastore
	org *organization.Organization
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

	cl = ginclient.New(ctx)
	cl.Defaults(func(r *http.Request) {})

	stripeApi.Route(cl.Router)
})

var _ = AfterSuite(func() {
	ctx.Close()
})

// func mockStripeDisputeEvent(event, status string) (*order.Order, *payment.Payment) {
// 	ord := order.New(db)
// 	ord.Put()

// 	pay := payment.New(db)
// 	pay.OrderId = ord.Id()
// 	pay.Amount = currency.Cents(1000)
// 	pay.Put()

// 	ord.PaymentIds = []string{pay.Id()}
// 	ord.Total = currency.Cents(1000)
// 	ord.Put()

// 	request := CreateDispute(event, status)
// 	w := client.PostRawJSON("/stripe/webhook", request)
// 	Expect(w.Code).To(Equal(200))

// 	pay2 := payment.New(db)
// 	pay2.GetById(pay.Id())

// 	ord2 := order.New(db)
// 	ord2.GetById(ord.Id())

// 	return ord2, pay2
// }

func mockStripeChargeEvent(event, status string, captured bool) (*order.Order, *payment.Payment) {
	refunded := false

	ord := order.New(db)
	ord.Put()

	pay := payment.New(db)
	pay.OrderId = ord.Id()
	pay.Amount = currency.Cents(1000)
	if status == "refunded" {
		refunded = true
	}
	pay.Put()

	ord.PaymentIds = []string{pay.Id()}
	ord.Total = currency.Cents(1000)
	ord.Put()

	request := CreatePayment(event, ord.Id(), pay.Id(), status, refunded, captured)
	cl.Post("/stripe/webhook", request, nil)

	pay2 := payment.New(db)
	ord2 := order.New(db)
	err := Retry(20, func() error {
		pay2.GetById(pay.Id())
		if pay.Status == pay2.Status {
			return errors.New("error")
		}

		ord2.GetById(ord.Id())
		return nil
	})
	Expect(err).NotTo(HaveOccurred())

	return ord2, pay2
}

var _ = Describe("Stripe Webhook Events", func() {
	Context("Respond To charge.updated Events", func() {
		It("Succeeded = true", func() {
			ord, pay := mockStripeChargeEvent("charge.updated", "succeeded", true)

			Expect(payment.Paid).To(Equal(string(pay.Status)))
			Expect(payment.Paid).To(Equal(string(ord.PaymentStatus)))
			Expect(order.Open).To(Equal(string(ord.Status)))

			Expect(ord.Paid).To(Equal(pay.Amount))
		})

		It("Status = failed", func() {
			ord, pay := mockStripeChargeEvent("charge.updated", "failed", true)

			Expect(payment.Failed).To(Equal(string(pay.Status)))
			Expect(payment.Failed).To(Equal(string(ord.PaymentStatus)))
			Expect(order.Cancelled).To(Equal(ord.Status))
		})

		It("Status = refunded", func() {
			ord, pay := mockStripeChargeEvent("charge.updated", "refunded", true)

			Expect(payment.Refunded).To(Equal(string(pay.Status)))
			Expect(payment.Refunded).To(Equal(string(ord.PaymentStatus)))
			Expect(order.Cancelled).To(Equal(ord.Status))
		})
	})

	// Context("Respond To charge.dispute.updated Events", func() {
	// 	It("Status = won", func() {
	// 		ord, pay := mockStripeChargeEvent("charge.dispute.updated", "won", true)
	// 		Expect(payment.Paid).To(Equal(string(pay.Status)))
	// 		Expect(payment.Paid).To(Equal(string(ord.PaymentStatus)))
	// 		Expect(order.Open).To(Equal(ord.Status))
	// 	})

	// 	It("Status = lost", func() {
	// 		ord, pay := mockStripeChargeEvent("charge.dispute.updated", "won", true)
	// 		Expect(payment.Refunded).To(Equal(string(pay.Status)))
	// 		Expect(payment.Refunded).To(Equal(string(ord.PaymentStatus)))
	// 		Expect(order.Cancelled).To(Equal(ord.Status))
	// 	})
	// })
})

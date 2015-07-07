package test

import (
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"crowdstart.com/datastore"
	"crowdstart.com/models/order"
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

	client = ginclient.New(ctx)
	client.Setup(func(r *http.Request) {})

	stripeApi.Route(client.Router)
})

var _ = AfterSuite(func() {
	ctx.Close()
})

func mockStripeEvent(event, status string, captured bool) (*order.Order, *payment.Payment) {
	ord := order.New(db)
	ord.Put()

	pay := payment.New(db)
	pay.OrderId = ord.Id()
	pay.Amount = currency.Cents(1000)
	pay.Put()

	ord.PaymentIds = []string{pay.Id()}
	ord.Put()

	request := CreateRequest(event, ord.Id(), pay.Id(), status, captured)
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
		})

		// It("Status = failed", func() {
		// 	ord, pay := mockStripeEvent("charge.updated", "failed", true)

		// 	Expect(payment.Cancelled).To(Equal(pay.Status))
		// 	Expect(payment.Cancelled).To(Equal(ord.PaymentStatus))
		// 	Expect(order.Cancelled).To(Equal(ord.Status))
		// })

		// It("Status = refunded", func() {
		// 	ord, pay := mockStripeEvent("charge.updated", "refunded", true)

		// 	Expect(payment.Refunded).To(Equal(pay.Status))
		// 	Expect(payment.Refunded).To(Equal(ord.PaymentStatus))
		// 	Expect(order.Cancelled).To(Equal(ord.Status))
		// })
	})
})

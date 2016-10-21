package test

import (
	"time"

	"github.com/stripe/stripe-go"

	"crowdstart.com/models/lineitem"
	"crowdstart.com/models/order"
	"crowdstart.com/models/payment"
	"crowdstart.com/models/product"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/models/variant"
	"crowdstart.com/util/json"
	"crowdstart.com/util/log"

	. "crowdstart.com/util/test/ginkgo"
)

// Create a mock stripe charge
func fakeCharge() *stripe.Charge {
	ch := new(stripe.Charge)
	ch.ID = "ch_000000000000000000000000"
	ch.Live = true
	ch.Meta = make(map[string]string)
	return ch
}

// Create a mock stripe event
func fakeEvent(name string, obj interface{}) *stripe.Event {
	ev := new(stripe.Event)
	ev.UserID = "1"
	ev.Live = true
	ev.Type = name
	ev.ID = "evt_000000000000000000000000"
	ev.Data = new(stripe.EventData)
	ev.Data.Raw = json.EncodeRaw(obj)
	return ev
}

var _ = Describe("Stripe Webhook", func() {
	var req *stripe.Event
	var ord *order.Order
	var pay *payment.Payment

	Before(func() {
		// Create fake product, variant
		prod := product.Fake(db)
		prod.MustCreate()
		vari := variant.Fake(db, prod.Id())
		vari.MustCreate()

		// Create fake order
		ord = order.Fake(db, lineitem.Fake(vari))

		// Create fake payment
		pay = payment.Fake(db)
		pay.Parent = ord.Key()
		pay.OrderId = ord.Id()
		pay.Amount = currency.Cents(ord.Total)
		ord.PaymentIds = []string{pay.Id()}

		// Save order
		ord.MustCreate()
		pay.MustCreate()
	})

	JustBefore(func() {
		cl.Post("/stripe/webhook", req, nil, 200)
	})

	Context("charge.updated Event", func() {
		Context("charge.status = succeeded", func() {
			Before(func() {
				ch := fakeCharge()
				ch.Paid = true
				ch.Status = "succeeded"
				ch.Amount = uint64(ord.Total)
				ch.Currency = stripe.Currency(ord.Currency)
				ch.Meta["order"] = ord.Id()
				ch.Meta["payment"] = pay.Id()
				req = fakeEvent("charge.updated", ch)
			})

			It("Should update payment", func() {
				time.Sleep(time.Second * 3)
				id := pay.Id()
				pay = payment.New(db)
				pay.GetById(id)
				log.JSON(pay)
			})
		})

		Context("charge.status = failed", func() {
			// ord, pay := mockStripeChargeEvent("charge.updated", "failed", true)
		})

		Context("charge.status = refunded", func() {
			// ord, pay := mockStripeChargeEvent("charge.updated", "refunded", true)
		})
	})
})

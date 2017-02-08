package test

import (
	"github.com/stripe/stripe-go"

	"hanzo.io/models/lineitem"
	"hanzo.io/models/order"
	"hanzo.io/models/payment"
	"hanzo.io/models/product"
	"hanzo.io/models/types/currency"
	"hanzo.io/models/variant"
	"hanzo.io/util/json"
	"hanzo.io/util/log"

	. "hanzo.io/util/test/ginkgo"
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

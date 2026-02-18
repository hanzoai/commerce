package test

import (
	stripe "github.com/stripe/stripe-go/v84"

	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/lineitem"
	"github.com/hanzoai/commerce/models/order"
	"github.com/hanzoai/commerce/models/payment"
	"github.com/hanzoai/commerce/models/product"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/models/variant"
	"github.com/hanzoai/commerce/util/json"

	. "github.com/hanzoai/commerce/util/test/ginkgo"
)

// Create a mock stripe charge
func fakeCharge() *stripe.Charge {
	ch := new(stripe.Charge)
	ch.ID = "ch_000000000000000000000000"
	ch.Livemode = true
	ch.Metadata = make(map[string]string)
	return ch
}

// Create a mock stripe event
func fakeEvent(name string, obj interface{}) *stripe.Event {
	ev := new(stripe.Event)
	ev.Account = "1"
	ev.Livemode = true
	ev.Type = name
	ev.ID = "evt_000000000000000000000000"
	ev.Data = &stripe.EventData{
		Raw: json.EncodeRaw(obj),
	}
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
				ch.Amount = int64(ord.Total)
				ch.Currency = stripe.Currency(ord.Currency)
				ch.Metadata["order"] = ord.Id()
				ch.Metadata["payment"] = pay.Id()
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

package test

import (
	"github.com/stripe/stripe-go/dispute"

	"crowdstart.com/models/payment"
	"crowdstart.com/thirdparty/stripe"
	"crowdstart.com/thirdparty/stripe/tasks"
	. "crowdstart.com/util/test/ginkgo"
)

var _ = Describe("thirdparty.stripe.UpdatePaymentFromDispute", func() {
	var construct = func() (*payment.Payment, *stripe.Dispute) {
		return new(payment.Payment), new(stripe.Dispute)
	}

	Context("When a dispute is won", func() {
		pay, dis := construct()
		dis.Status = dispute.Won
		It("should mark the payment as Paid", func() {
			tasks.UpdatePaymentFromDispute(pay, dis)
			Expect(pay.Status == payment.Paid)
		})
	})
})

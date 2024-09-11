package test

import (
	"hanzo.io/models/payment"
	"hanzo.io/thirdparty/stripe"
	"hanzo.io/thirdparty/stripe/tasks"
	. "hanzo.io/util/test/ginkgo"
)

// Ginkgo doesn't understand string enums. As a result, `pay.Status`
// needs to be cast to string.
var _ = Describe("thirdparty.stripe.UpdatePaymentFromDispute", func() {
	var construct = func() (*payment.Payment, *stripe.Dispute) {
		return new(payment.Payment), new(stripe.Dispute)
	}

	Context("When a dispute is won", func() {
		pay, dispute := construct()
		dispute.Status = stripe.Won
		It("should mark the payment as Paid", func() {
			tasks.UpdatePaymentFromDispute(pay, dispute)
			Expect(pay.Status).To(Equal(payment.Paid))
		})
	})

	Context("When the charge of a dispute is refunded", func() {
		pay, dispute := construct()
		dispute.Status = stripe.Needsresponse
		It("should mark the payment as Refunded", func() {
			tasks.UpdatePaymentFromDispute(pay, dispute)
			Expect(pay.Status).To(Equal(payment.Refunded))
		})
	})

	Context("When a dispute is lost (or anything else)", func() {
		It("should mark the payment as Disputed", func() {
			pay, dispute := construct()

			dispute.Status = stripe.Lost
			tasks.UpdatePaymentFromDispute(pay, dispute)
			Expect(pay.Status).To(Equal(payment.Disputed))

			dispute.Status = stripe.Review
			tasks.UpdatePaymentFromDispute(pay, dispute)
			Expect(pay.Status).To(Equal(payment.Disputed))
		})
	})
})

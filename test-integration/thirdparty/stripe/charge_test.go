package test

import (
	"github.com/hanzoai/commerce/models/payment"
	"github.com/hanzoai/commerce/thirdparty/stripe"
	"github.com/hanzoai/commerce/thirdparty/stripe/tasks"
	. "github.com/hanzoai/commerce/util/test/ginkgo"
)

var _ = Describe("thirdparty.stripe.UpdatePaymentFromCharge", func() {
	var construct = func() (*payment.Payment, *stripe.Charge) {
		return new(payment.Payment), new(stripe.Charge)
	}

	Context("When a charge is captured", func() {
		pay, charge := construct()
		charge.Captured = true
		charge.Status = "success"
		It("should mark the payment as Paid", func() {
			tasks.UpdatePaymentFromCharge(pay, charge)
			Expect(pay.Status).To(Equal(payment.Paid))
		})
	})

	Context("When a charge is refunded", func() {
		pay, charge := construct()
		charge.Refunded = true
		It("should mark the payment as Paid", func() {
			tasks.UpdatePaymentFromCharge(pay, charge)
			Expect(pay.Status).To(Equal(payment.Refunded))
		})
	})

	Context("When a charge is paid", func() {
		pay, charge := construct()
		charge.Paid = true
		charge.Captured = true
		charge.Status = "success"
		It("should mark the payment as Paid", func() {
			tasks.UpdatePaymentFromCharge(pay, charge)
			Expect(pay.Status).To(Equal(payment.Paid))
		})
	})

	Context("For every other state", func() {
		It("should mark the payment as Unpaid", func() {
			pay, charge := construct()
			tasks.UpdatePaymentFromCharge(pay, charge)
			Expect(pay.Status).To(Equal(payment.Unpaid))
		})
	})
})

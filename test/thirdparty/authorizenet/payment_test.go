package test

import (
	"hanzo.io/models/payment"
	"hanzo.io/models/types/currency"
	"hanzo.io/thirdparty/authorizenet"

	. "hanzo.io/util/test/ginkgo"
)

var _ = Describe("thirdparty.authorizenet.payments", func() {

	Context("Payments", func() {
		It("Should succeed to authorize", func() {
			pay := payment.Fake(db)
			retPay, err := client.Authorize(pay)
			Expect(err).ToNot(HaveOccurred())
			Expect(retPay.Account.TransId).NotTo(BeNil())
			Expect(retPay.Account.TransId).NotTo(Equal(""))
		})
		It("Should succeed to charge an authorized payment", func() {
			pay := payment.Fake(db)
			retPay, err := client.Authorize(pay)
			Expect(err).ToNot(HaveOccurred())
			capPay, err := client.Capture(retPay)
			Expect(err).ToNot(HaveOccurred())
			Expect(capPay.Account.TransId).NotTo(BeNil())
			Expect(capPay.Account.TransId).NotTo(Equal(""))
		})
		It("Should succeed a one-step charge", func() {
			pay := payment.Fake(db)
			chrgPay, err := client.Charge(pay)
			Expect(err).ToNot(HaveOccurred())
			Expect(chrgPay.Account.TransId).NotTo(BeNil())
			Expect(chrgPay.Account.TransId).NotTo(Equal(""))
		})
		It("Should succeed at a refund", func() {
			pay := payment.Fake(db)
			chrgPay, err := client.Charge(pay)
			Expect(err).ToNot(HaveOccurred())
			Expect(chrgPay.Account.TransId).NotTo(BeNil())
			Expect(chrgPay.Account.TransId).NotTo(Equal(""))
			_, err = client.RefundPayment(pay, currency.Cents(50))
			Expect(err).To(Equal(authorizenet.MinimumRefundTimeNotReachedError))
			// AUthorize.net only allows settled transactions to be refunded.
			// That usually means the next day.
			// We can't test the happy path, but we can ensure that the API
			// At least understood our request.
		})
	})
})

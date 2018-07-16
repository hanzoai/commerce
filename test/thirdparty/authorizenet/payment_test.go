package test

import (
	"hanzo.io/models/payment"
	//"hanzo.io/thirdparty/authorizenet"
	. "hanzo.io/util/test/ginkgo"
)

var _ = Describe("thirdparty.authorizenet.authorize", func() {

	Context("Authorize a payment", func() {
		It("Should succeed to authorize", func() {
			pay := payment.Fake(db)
			retPay, err := client.Authorize(pay)
			Expect(err).ToNot(HaveOccurred())
			Expect(retPay.Account.TransId).NotTo(BeNil())
			Expect(retPay.Account.TransId).NotTo(Equal(""))
		})
		It("Should succeed to an authorized payment", func() {
			pay := payment.Fake(db)
			retPay, err := client.Authorize(pay)
			Expect(err).ToNot(HaveOccurred())
			capPay, err := client.Capture(retPay)
			Expect(err).ToNot(HaveOccurred())
			Expect(capPay.Account.TransId).NotTo(BeNil())
			Expect(capPay.Account.TransId).NotTo(Equal(""))
		})
	})
})


package test

import (
	"hanzo.io/models/payment"
	//"hanzo.io/thirdparty/authorizenet"
	. "hanzo.io/util/test/ginkgo"
)

var _ = Describe("thirdparty.authorizenet.authorize", func() {

	Context("Authorize a payment", func() {
		pay := payment.Fake(db)
		It("Should succed to authorize", func() {
			retPay, err := client.Authorize(pay)
			Expect(err).ToNot(HaveOccurred())
			Expect(retPay.Account.TransId).NotTo(BeNil())
			Expect(retPay.Account.TransId).NotTo(Equal(""))
		})
	})
})


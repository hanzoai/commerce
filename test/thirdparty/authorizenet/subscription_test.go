package test

import (
	"hanzo.io/models/subscription"
	//"hanzo.io/thirdparty/authorizenet"
	. "hanzo.io/util/test/ginkgo"
)

var _ = Describe("thirdparty.authorizenet.subscription", func() {
	Context("Subscription", func() {
		It("Should make new subscription", func() {
			sub := subscription.Fake(db)
			retSub, err := client.NewSubscription(sub)
			Expect(err).ToNot(HaveOccurred())
			Expect(retSub.Account.TransId).NotTo(BeNil())
			Expect(retSub.Account.TransId).NotTo(Equal(""))
		})
	})
})

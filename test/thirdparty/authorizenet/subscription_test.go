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
			Expect(retSub.Ref.AuthorizeNet.SubscriptionId).NotTo(Equal(""))
			Expect(retSub.Ref.AuthorizeNet.CustomerProfileId).NotTo(Equal(""))
			Expect(retSub.Ref.AuthorizeNet.CustomerPaymentProfileId).NotTo(Equal(""))
		})
		It("Should cancel subscription", func() {
			sub := subscription.Fake(db)
			retSub, err := client.NewSubscription(sub)
			Expect(err).ToNot(HaveOccurred())
			canceledSub, err := client.CancelSubscription(retSub)
			Expect(err).ToNot(HaveOccurred())
			Expect(canceledSub.Canceled).To(BeTrue())
		})
	})
})

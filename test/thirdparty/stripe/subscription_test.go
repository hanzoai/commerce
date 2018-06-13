package test

import (
	"hanzo.io/models/subscription"
	"hanzo.io/models/user"
	"hanzo.io/thirdparty/stripe"
	"hanzo.io/thirdparty/stripe/tasks"
	. "hanzo.io/util/test/ginkgo"
)

var _ = Describe("thirdparty.stripe.SubscriptionApiTest", func() {

	Context("New Subscription", func() {
		sub := subscription.Fake()
		usr := user.Fake()
		It("should create new subscription", func() {
			stripeSub, err := client.NewSubscription(token, usr, sub)
			Expect(err).To(BeNil())
		})
	})

})

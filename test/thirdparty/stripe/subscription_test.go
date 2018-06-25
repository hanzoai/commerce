package test

import (
	"hanzo.io/models/subscription"
	"hanzo.io/models/user"
	"hanzo.io/log"
	. "hanzo.io/util/test/ginkgo"
)

var _ = FDescribe("thirdparty.stripe.SubscriptionApiTest", func() {
	Context("New Subscription", func() {
		It("should create new subscription", func() {
			sub := subscription.Fake(db)
			usr := user.Fake(db)

			log.JSON("USER %v", usr.Id())
			_, err := client.NewSubscription(token, usr, sub)
			Expect(err).To(BeNil())
		})
	})
})

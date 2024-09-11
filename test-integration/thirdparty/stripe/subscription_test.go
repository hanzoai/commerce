package test

import (
	"hanzo.io/log"
	"hanzo.io/models/plan"
	"hanzo.io/models/subscription"
	"hanzo.io/models/user"
	. "hanzo.io/util/test/ginkgo"
)

var _ = Describe("thirdparty.stripe.SubscriptionApiTest", func() {
	Context("New Subscription", func() {
		It("should create new plan and subscription", func() {
			pln := plan.Fake(db)
			sub := subscription.Fake(db)
			usr := user.Fake(db)

			sub.Plan = *pln

			tok, err := client.AuthorizeSubscription(sub)
			Expect(err).To(BeNil())

			cust, err := client.NewCustomer(tok.ID, usr)
			log.JSON(usr)
			Expect(err).To(BeNil())

			spln, err := client.NewPlan(pln)
			log.JSON(pln)
			Expect(err).To(BeNil())
			Expect(spln.ID).To(Equal(pln.Id()))
			Expect(spln.ID).To(Equal(pln.Ref.Stripe.Id))

			_, err = client.NewSubscription(cust, sub)
			log.JSON(sub)
			Expect(err).To(BeNil())
		})
	})
})

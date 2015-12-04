package test

import (
	"crowdstart.com/models/plan"
	"crowdstart.com/models/subscription"
	"crowdstart.com/models/user"
	"crowdstart.com/util/log"

	. "crowdstart.com/util/test/ginkgo"
)

func stripeVerifyUser(usr *user.User) {
	c, err := sc.Customers.Get(usr.Accounts.Stripe.CustomerId, nil)
	log.Debug("StripeVerifyUser Results:\n%v\n%v", c, err)
	Expect(c).ToNot(BeNil())
	Expect(err).ToNot(HaveOccurred())
}

func stripeVerifyPlan(pln *plan.Plan) {
	c, err := sc.Plans.Get(pln.StripeId, nil)
	log.Debug("StripeVerifyPlan Results:\n%v\n%v", c, err)
	Expect(c).ToNot(BeNil())
	Expect(err).ToNot(HaveOccurred())
}

func stripeVerifySubscriptions(sub *subscription.Subscription) {
	c, err := sc.Subs.Get(sub.Account.SubscriptionId, nil)
	log.Debug("StripeVerifySubscriptions Results:\n%v\n%v", c, err)
	Expect(c).ToNot(BeNil())
	Expect(err).ToNot(HaveOccurred())
}

func stripeVerifyCards(usr *user.User, cardIds []string) {
	c, err := sc.Customers.Get(usr.Accounts.Stripe.CustomerId, nil)
	Expect(c).ToNot(BeNil())
	Expect(err).ToNot(HaveOccurred())

	Expect(len(c.Sources.Values)).To(Equal(len(cardIds)))
	for i, source := range c.Sources.Values {
		Expect(source.Card.ID).To(Equal(cardIds[i]))
	}

	log.Debug("StripeVerifyCard Results:\n%v\n%v", c, err)
}

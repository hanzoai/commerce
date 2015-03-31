package test

import (
	"crowdstart.io/models2/user"
	"crowdstart.io/util/log"

	. "crowdstart.io/util/test/ginkgo"
)

func stripeVerifyUser(usr *user.User) {
	c, err := sc.Customers.Get(usr.Accounts.Stripe.CustomerId, nil)
	log.Debug("StripeVerifyUser Results:\n%v\n%v", c, err)
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

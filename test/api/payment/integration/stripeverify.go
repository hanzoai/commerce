package test

import (
	"crowdstart.io/models2/payment"
	"crowdstart.io/models2/user"
	"crowdstart.io/util/log"

	. "crowdstart.io/util/test/ginkgo"
)

func stripeVerifyCharge(pay *payment.Payment) {
	c, err := sc.Charges.Get(pay.ChargeId, nil)
	Expect(c).ToNot(BeNil())
	Expect(err).ToNot(HaveOccurred())

	Expect(c.Captured).To(BeTrue())
	log.Debug("StripeVerifyCharge Results:\n%v\n%v", c, err)
}

func stripeVerifyAuth(pay *payment.Payment) {
	c, err := sc.Charges.Get(pay.ChargeId, nil)
	Expect(c).ToNot(BeNil())
	Expect(err).ToNot(HaveOccurred())

	Expect(c.Captured).To(BeFalse())
	log.Debug("StripeVerifyAuth Results:\n%v\n%v", c, err)
}

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

package test

import (
	"hanzo.io/models/payment"
	"hanzo.io/models/user"
	"hanzo.io/thirdparty/stripe"
	"hanzo.io/util/log"

	. "hanzo.io/util/test/ginkgo"
)

func stripeVerifyCharge(pay *payment.Payment) {
	sc := stripe.New(ctx, org.Stripe.Test.AccessToken)
	c, err := sc.Charges.Get(pay.Account.ChargeId, nil)
	Expect(c).ToNot(BeNil())
	Expect(err).ToNot(HaveOccurred())

	Expect(c.Captured).To(BeTrue())
	log.Debug("StripeVerifyCharge Results:\n%v\n%v", c, err)
}

func stripeVerifyAuth(pay *payment.Payment) {
	sc := stripe.New(ctx, org.Stripe.Test.AccessToken)
	c, err := sc.Charges.Get(pay.Account.ChargeId, nil)
	Expect(c).ToNot(BeNil())
	Expect(err).ToNot(HaveOccurred())

	Expect(c.Captured).To(BeFalse())
	log.Debug("StripeVerifyAuth Results:\n%v\n%v", c, err)
}

func stripeVerifyUser(usr *user.User) {
	sc := stripe.New(ctx, org.Stripe.Test.AccessToken)
	c, err := sc.Customers.Get(usr.Accounts.Stripe.CustomerId, nil)
	log.Debug("StripeVerifyUser Results:\n%v\n%v", c, err)
	Expect(c).ToNot(BeNil())
	Expect(err).ToNot(HaveOccurred())
}

func stripeVerifyCards(usr *user.User, cardIds []string) {
	sc := stripe.New(ctx, org.Stripe.Test.AccessToken)
	c, err := sc.Customers.Get(usr.Accounts.Stripe.CustomerId, nil)
	Expect(c).ToNot(BeNil())
	Expect(err).ToNot(HaveOccurred())

	Expect(len(c.Sources.Values)).To(Equal(len(cardIds)))
	for i, source := range c.Sources.Values {
		Expect(source.Card.ID).To(Equal(cardIds[i]))
	}

	log.Debug("StripeVerifyCard Results:\n%v\n%v", c, err)
}

package stripe_test

import (
	"hanzo.io/models/payment"
	"hanzo.io/models/user"
	"hanzo.io/log"

	. "hanzo.io/util/test/ginkgo"
)

func stripeVerifyCharge(pay *payment.Payment) {
	ch, err := sc.Charges.Get(pay.Account.ChargeId, nil)
	Expect1(ch).ToNot(BeNil())
	Expect1(err).ToNot(HaveOccurred())
	Expect1(ch.Captured).To(BeTrue())
	log.Debug("StripeVerifyCharge Results:\n%v\n%v", ch, err)
}

func stripeVerifyAuth(pay *payment.Payment) {
	ch, err := sc.Charges.Get(pay.Account.ChargeId, nil)
	Expect1(ch).ToNot(BeNil())
	Expect1(err).ToNot(HaveOccurred())

	Expect1(ch.Captured).To(BeFalse())
	log.Debug("StripeVerifyAuth Results:\n%v\n%v", ch, err)
}

func stripeVerifyUser(usr *user.User) {
	cust, err := sc.Customers.Get(usr.Accounts.Stripe.CustomerId, nil)
	log.Debug("StripeVerifyUser Results:\n%v\n%v", cust, err)
	Expect1(cust).ToNot(BeNil())
	Expect1(err).ToNot(HaveOccurred())
}

func stripeVerifyCards(usr *user.User, cardIds []string) {
	cust, err := sc.Customers.Get(usr.Accounts.Stripe.CustomerId, nil)
	Expect1(cust).ToNot(BeNil())
	Expect1(err).ToNot(HaveOccurred())

	sources := make([]string, 0)
	for _, source := range cust.Sources.Values {
		sources = append(sources, source.Card.ID)
	}

	log.Debug("StripeVerifyCards Expected: %v\nGot:%v", cardIds, sources)
	Expect1(len(cust.Sources.Values)).To(Equal(len(cardIds)))
	Expect1(sources).To(ConsistOf(cardIds))
}

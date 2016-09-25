package test

import (
	"crowdstart.com/models/payment"
	"crowdstart.com/models/user"
	"crowdstart.com/util/log"

	. "crowdstart.com/util/test/ginkgo"
)

func stripeVerifyCharge(pay *payment.Payment) {
	ch, err := sc.Charges.Get(pay.Account.ChargeId, nil)
	Expect(ch).ToNot(BeNil())
	Expect(err).ToNot(HaveOccurred())
	Expect(ch.Captured).To(BeTrue())
	log.Debug("StripeVerifyCharge Results:\n%v\n%v", ch, err)
}

func stripeVerifyAuth(pay *payment.Payment) {
	ch, err := sc.Charges.Get(pay.Account.ChargeId, nil)
	Expect(ch).ToNot(BeNil())
	Expect(err).ToNot(HaveOccurred())

	Expect(ch.Captured).To(BeFalse())
	log.Debug("StripeVerifyAuth Results:\n%v\n%v", ch, err)
}

func stripeVerifyUser(usr *user.User) {
	cust, err := sc.Customers.Get(usr.Accounts.Stripe.CustomerId, nil)
	log.Debug("StripeVerifyUser Results:\n%v\n%v", cust, err)
	Expect(cust).ToNot(BeNil())
	Expect(err).ToNot(HaveOccurred())
}

func stripeVerifyCards(usr *user.User, cardIds []string) {
	cust, err := sc.Customers.Get(usr.Accounts.Stripe.CustomerId, nil)
	Expect(cust).ToNot(BeNil())
	Expect(err).ToNot(HaveOccurred())

	sources := make([]string, 0)
	for _, source := range cust.Sources.Values {
		sources = append(sources, source.Card.ID)
	}

	log.Debug("StripeVerifyCards Expected: %v\nGot:%v", cardIds, sources)
	Expect(len(cust.Sources.Values)).To(Equal(len(cardIds)))
	Expect(sources).To(ConsistOf(cardIds))
}

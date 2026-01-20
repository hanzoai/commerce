package integration

import (
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/payment"
	"github.com/hanzoai/commerce/models/user"
	"github.com/hanzoai/commerce/thirdparty/stripe"

	. "github.com/hanzoai/commerce/util/test/ginkgo"
)

func stripeVerifyCharge(pay *payment.Payment) {
	sc := stripe.New(ctx, org.Stripe.Test.AccessToken)

	ch, err := sc.Charges.Get(pay.Account.ChargeId, nil)
	Expect1(ch).ToNot(BeNil())
	Expect1(err).ToNot(HaveOccurred())
	Expect1(ch.Captured).To(BeTrue())
	log.Debug("StripeVerifyCharge Results:\n%v\n%v", ch, err)
}

func stripeVerifyAuth(pay *payment.Payment) {
	sc := stripe.New(ctx, org.Stripe.Test.AccessToken)

	ch, err := sc.Charges.Get(pay.Account.ChargeId, nil)
	Expect1(ch).ToNot(BeNil())
	Expect1(err).ToNot(HaveOccurred())

	Expect1(ch.Captured).To(BeFalse())
	log.Debug("StripeVerifyAuth Results:\n%v\n%v", ch, err)
}

func stripeVerifyUser(usr *user.User) {
	sc := stripe.New(ctx, org.Stripe.Test.AccessToken)

	cust, err := sc.Customers.Get(usr.Accounts.Stripe.CustomerId, nil)
	log.Debug("StripeVerifyUser Results:\n%v\n%v", cust, err)
	Expect1(cust).ToNot(BeNil())
	Expect1(err).ToNot(HaveOccurred())
}

func stripeVerifyCards(usr *user.User, cardIds []string) {
	sc := stripe.New(ctx, org.Stripe.Test.AccessToken)

	cust, err := sc.Customers.Get(usr.Accounts.Stripe.CustomerId, nil)
	Expect1(cust).ToNot(BeNil())
	Expect1(err).ToNot(HaveOccurred())

	sources := make([]string, 0)
	for _, source := range cust.Sources.Data {
		if source.Type == "card" {
			sources = append(sources, source.ID)
		}
	}

	log.Debug("StripeVerifyCards Expected: %v\nGot:%v", cardIds, sources)
	Expect1(len(sources)).To(Equal(len(cardIds)))
	Expect1(sources).To(ConsistOf(cardIds))
}

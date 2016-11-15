package test

import (
	"testing"

	"crowdstart.com/datastore"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/referral"
	"crowdstart.com/models/referralprogram"
	"crowdstart.com/models/referrer"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/models/user"
	"crowdstart.com/util/test/ae"

	. "crowdstart.com/util/test/ginkgo"
)

func Test(t *testing.T) {
	Setup("models/referrer", t)
}

var (
	ctx  ae.Context
	db   *datastore.Datastore
	nsDb *datastore.Datastore
)

// Setup appengine context and datastore before tests
var _ = BeforeSuite(func() {
	ctx = ae.NewContext()
	db = datastore.New(ctx)
})

// Tear-down appengine context
var _ = AfterSuite(func() {
	ctx.Close()
})

var _ = Describe("Referrer", func() {
	Context("SaveReferral", func() {
		var org *organization.Organization

		Before(func() {
			org = organization.New(db)
			org.MustCreate()

		})

		It("should work with referral triggers", func() {
			usr := user.Fake(db)
			usr.MustCreate()

			prog := referralprogram.New(db)
			prog.Trigger.Type = referralprogram.ReferralsGreaterThan
			prog.Trigger.ReferralsGreaterThan = 0
			prog.Actions = []referralprogram.Action{
				referralprogram.Action{
					Type: referralprogram.StoreCredit,
					CreditAction: referralprogram.CreditAction{
						Currency: currency.USD,
						Amount:   currency.Cents(7),
					},
				},
			}
			prog.MustCreate()

			rfr := referrer.Fake(db, usr.Id())
			rfr.ProgramId = prog.Id()
			rfr.MustCreate()

			rfl, err := rfr.SaveReferral(ctx, org.Id(), referral.NewOrder, usr)
			Expect(err).ToNot(HaveOccurred())
			rfl.MustCreate()

			err = usr.LoadReferrals()
			Expect(err).ToNot(HaveOccurred())
			Expect(len(usr.Referrals)).To(Equal(1))

			err = usr.CalculateBalances()
			Expect(err).ToNot(HaveOccurred())
			Expect(usr.Balances[currency.USD]).To(Equal(currency.Cents(7)))

			rfl, err = rfr.SaveReferral(ctx, org.Id(), referral.NewOrder, usr)
			Expect(err).ToNot(HaveOccurred())
			rfl.MustCreate()

			err = usr.LoadReferrals()
			Expect(err).ToNot(HaveOccurred())
			Expect(len(usr.Referrals)).To(Equal(2))

			err = usr.CalculateBalances()
			Expect(err).ToNot(HaveOccurred())
			Expect(usr.Balances[currency.USD]).To(Equal(currency.Cents(14)))
		})

		It("should not fire everytime for referral triggers", func() {
			usr := user.Fake(db)
			usr.MustCreate()

			prog := referralprogram.New(db)
			prog.Trigger.Type = referralprogram.ReferralsGreaterThan
			prog.Trigger.ReferralsGreaterThan = 1
			prog.Actions = []referralprogram.Action{
				referralprogram.Action{
					Type: referralprogram.StoreCredit,
					CreditAction: referralprogram.CreditAction{
						Currency: currency.USD,
						Amount:   currency.Cents(7),
					},
				},
			}
			prog.MustCreate()

			rfr := referrer.Fake(db, usr.Id())
			rfr.ProgramId = prog.Id()
			rfr.MustCreate()

			rfl, err := rfr.SaveReferral(ctx, org.Id(), referral.NewOrder, usr)
			Expect(err).ToNot(HaveOccurred())
			rfl.MustCreate()

			err = usr.LoadReferrals()
			Expect(err).ToNot(HaveOccurred())
			Expect(len(usr.Referrals)).To(Equal(1))

			err = usr.CalculateBalances()
			Expect(err).ToNot(HaveOccurred())
			Expect(usr.Balances[currency.USD]).To(Equal(currency.Cents(0)))

			rfl, err = rfr.SaveReferral(ctx, org.Id(), referral.NewOrder, usr)
			Expect(err).ToNot(HaveOccurred())
			rfl.MustCreate()

			err = usr.LoadReferrals()
			Expect(err).ToNot(HaveOccurred())
			Expect(len(usr.Referrals)).To(Equal(2))

			err = usr.CalculateBalances()
			Expect(err).ToNot(HaveOccurred())
			Expect(usr.Balances[currency.USD]).To(Equal(currency.Cents(7)))
		})

		It("should work with referral triggers once", func() {
			usr := user.Fake(db)
			usr.MustCreate()

			prog := referralprogram.New(db)
			prog.Trigger.Type = referralprogram.ReferralsGreaterThan
			prog.Trigger.ReferralsGreaterThan = 0
			prog.Actions = []referralprogram.Action{
				referralprogram.Action{
					Type: referralprogram.StoreCredit,
					Once: true,
					CreditAction: referralprogram.CreditAction{
						Currency: currency.USD,
						Amount:   currency.Cents(7),
					},
				},
			}
			prog.MustCreate()

			rfr := referrer.Fake(db, usr.Id())
			rfr.ProgramId = prog.Id()
			rfr.MustCreate()

			rfl, err := rfr.SaveReferral(ctx, org.Id(), referral.NewOrder, usr)
			Expect(err).ToNot(HaveOccurred())
			rfl.MustCreate()

			err = usr.LoadReferrals()
			Expect(err).ToNot(HaveOccurred())
			Expect(len(usr.Referrals)).To(Equal(1))

			err = usr.CalculateBalances()
			Expect(err).ToNot(HaveOccurred())
			Expect(usr.Balances[currency.USD]).To(Equal(currency.Cents(7)))

			rfl, err = rfr.SaveReferral(ctx, org.Id(), referral.NewOrder, usr)
			Expect(err).ToNot(HaveOccurred())
			rfl.MustCreate()

			err = usr.LoadReferrals()
			Expect(err).ToNot(HaveOccurred())
			Expect(len(usr.Referrals)).To(Equal(2))

			err = usr.CalculateBalances()
			Expect(err).ToNot(HaveOccurred())
			Expect(usr.Balances[currency.USD]).To(Equal(currency.Cents(7)))
		})

		It("should work with balance triggers", func() {
			usr := user.Fake(db)
			usr.MustCreate()

			prog := referralprogram.New(db)
			prog.Trigger.Type = referralprogram.CreditGreaterThan
			prog.Trigger.CreditGreaterThan = 6
			prog.Trigger.Currency = currency.USD
			prog.Actions = []referralprogram.Action{
				referralprogram.Action{
					Type: referralprogram.StoreCredit,
					CreditAction: referralprogram.CreditAction{
						Currency: currency.USD,
						Amount:   currency.Cents(7),
					},
				},
			}
			prog.MustCreate()

			rfr := referrer.Fake(db, usr.Id())
			rfr.ProgramId = prog.Id()
			rfr.MustCreate()

			rfl, err := rfr.SaveReferral(ctx, org.Id(), referral.NewOrder, usr)
			Expect(err).ToNot(HaveOccurred())
			rfl.MustCreate()

			err = usr.LoadReferrals()
			Expect(err).ToNot(HaveOccurred())
			Expect(len(usr.Referrals)).To(Equal(1))

			err = usr.CalculateBalances()
			Expect(err).ToNot(HaveOccurred())
			Expect(usr.Balances[currency.USD]).To(Equal(currency.Cents(7)))

			rfl, err = rfr.SaveReferral(ctx, org.Id(), referral.NewOrder, usr)
			Expect(err).ToNot(HaveOccurred())
			rfl.MustCreate()

			err = usr.LoadReferrals()
			Expect(err).ToNot(HaveOccurred())
			Expect(len(usr.Referrals)).To(Equal(2))

			err = usr.CalculateBalances()
			Expect(err).ToNot(HaveOccurred())
			Expect(usr.Balances[currency.USD]).To(Equal(currency.Cents(14)))
		})

		It("should not fire everytime for balance triggers", func() {
			usr := user.Fake(db)
			usr.MustCreate()

			prog := referralprogram.New(db)
			prog.Trigger.Type = referralprogram.CreditGreaterThan
			prog.Trigger.CreditGreaterThan = 7
			prog.Trigger.Currency = currency.USD
			prog.Actions = []referralprogram.Action{
				referralprogram.Action{
					Type: referralprogram.StoreCredit,
					CreditAction: referralprogram.CreditAction{
						Currency: currency.USD,
						Amount:   currency.Cents(7),
					},
				},
			}
			prog.MustCreate()

			rfr := referrer.Fake(db, usr.Id())
			rfr.ProgramId = prog.Id()
			rfr.MustCreate()

			rfl, err := rfr.SaveReferral(ctx, org.Id(), referral.NewOrder, usr)
			Expect(err).ToNot(HaveOccurred())
			rfl.MustCreate()

			err = usr.LoadReferrals()
			Expect(err).ToNot(HaveOccurred())
			Expect(len(usr.Referrals)).To(Equal(1))

			err = usr.CalculateBalances()
			Expect(err).ToNot(HaveOccurred())
			Expect(usr.Balances[currency.USD]).To(Equal(currency.Cents(0)))

			rfl, err = rfr.SaveReferral(ctx, org.Id(), referral.NewOrder, usr)
			Expect(err).ToNot(HaveOccurred())
			rfl.MustCreate()

			err = usr.LoadReferrals()
			Expect(err).ToNot(HaveOccurred())
			Expect(len(usr.Referrals)).To(Equal(2))

			err = usr.CalculateBalances()
			Expect(err).ToNot(HaveOccurred())
			Expect(usr.Balances[currency.USD]).To(Equal(currency.Cents(0)))
		})

		It("should work with balance triggers once", func() {
			usr := user.Fake(db)
			usr.MustCreate()

			prog := referralprogram.New(db)
			prog.Trigger.Type = referralprogram.CreditGreaterThan
			prog.Trigger.CreditGreaterThan = 6
			prog.Trigger.Currency = currency.USD
			prog.Actions = []referralprogram.Action{
				referralprogram.Action{
					Type: referralprogram.StoreCredit,
					Once: true,
					CreditAction: referralprogram.CreditAction{
						Currency: currency.USD,
						Amount:   currency.Cents(7),
					},
				},
			}
			prog.MustCreate()

			rfr := referrer.Fake(db, usr.Id())
			rfr.ProgramId = prog.Id()
			rfr.MustCreate()

			rfl, err := rfr.SaveReferral(ctx, org.Id(), referral.NewOrder, usr)
			Expect(err).ToNot(HaveOccurred())
			rfl.MustCreate()

			err = usr.LoadReferrals()
			Expect(err).ToNot(HaveOccurred())
			Expect(len(usr.Referrals)).To(Equal(1))

			err = usr.CalculateBalances()
			Expect(err).ToNot(HaveOccurred())
			Expect(usr.Balances[currency.USD]).To(Equal(currency.Cents(7)))

			rfl, err = rfr.SaveReferral(ctx, org.Id(), referral.NewOrder, usr)
			Expect(err).ToNot(HaveOccurred())
			rfl.MustCreate()

			err = usr.LoadReferrals()
			Expect(err).ToNot(HaveOccurred())
			Expect(len(usr.Referrals)).To(Equal(2))

			err = usr.CalculateBalances()
			Expect(err).ToNot(HaveOccurred())
			Expect(usr.Balances[currency.USD]).To(Equal(currency.Cents(7)))
		})

		// Deprecate soon
		It("should work with old balance triggers", func() {
			usr := user.Fake(db)
			usr.MustCreate()

			prog := referralprogram.New(db)
			prog.Triggers = []int{0}
			prog.Actions = []referralprogram.Action{
				referralprogram.Action{
					Type: referralprogram.StoreCredit,
					CreditAction: referralprogram.CreditAction{
						Currency: currency.USD,
						Amount:   currency.Cents(7),
					},
				},
			}
			prog.MustCreate()

			rfr := referrer.Fake(db, usr.Id())
			rfr.ProgramId = prog.Id()
			rfr.MustCreate()

			rfl, err := rfr.SaveReferral(ctx, org.Id(), referral.NewOrder, usr)
			Expect(err).ToNot(HaveOccurred())
			rfl.MustCreate()

			err = usr.LoadReferrals()
			Expect(err).ToNot(HaveOccurred())
			Expect(len(usr.Referrals)).To(Equal(1))

			err = usr.CalculateBalances()
			Expect(err).ToNot(HaveOccurred())
			Expect(usr.Balances[currency.USD]).To(Equal(currency.Cents(7)))

			rfl, err = rfr.SaveReferral(ctx, org.Id(), referral.NewOrder, usr)
			Expect(err).ToNot(HaveOccurred())
			rfl.MustCreate()

			err = usr.LoadReferrals()
			Expect(err).ToNot(HaveOccurred())
			Expect(len(usr.Referrals)).To(Equal(2))

			err = usr.CalculateBalances()
			Expect(err).ToNot(HaveOccurred())
			Expect(usr.Balances[currency.USD]).To(Equal(currency.Cents(14)))
		})

	})
})

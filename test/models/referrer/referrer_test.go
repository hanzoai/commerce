package test

import (
	"testing"

	"hanzo.io/datastore"
	"hanzo.io/models/organization"
	"hanzo.io/models/referral"
	"hanzo.io/models/referralprogram"
	"hanzo.io/models/referrer"
	"hanzo.io/models/types/currency"
	"hanzo.io/models/user"
	"hanzo.io/util/test/ae"

	. "hanzo.io/util/test/ginkgo"
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
			prog.Actions = []referralprogram.Action{
				referralprogram.Action{
					Trigger: referralprogram.Trigger{
						Type: referralprogram.ReferralsGreaterThanOrEquals,
						ReferralsGreaterThanOrEqualsTrigger: referralprogram.ReferralsGreaterThanOrEqualsTrigger{
							ReferralsGreaterThanOrEquals: 1,
						},
					},
					Name: "test",
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

		It("should work with multiple referral triggers", func() {
			usr := user.Fake(db)
			usr.MustCreate()

			prog := referralprogram.New(db)
			prog.Actions = []referralprogram.Action{
				referralprogram.Action{
					Trigger: referralprogram.Trigger{
						Type: referralprogram.Always,
					},
					Name: "test",
					Type: referralprogram.StoreCredit,
					CreditAction: referralprogram.CreditAction{
						Currency: currency.USD,
						Amount:   currency.Cents(7),
					},
				},
				referralprogram.Action{
					Trigger: referralprogram.Trigger{
						Type: referralprogram.Always,
					},
					Name: "test2",
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
			Expect(usr.Balances[currency.USD]).To(Equal(currency.Cents(14)))
		})

		It("should not fire everytime for referral triggers", func() {
			usr := user.Fake(db)
			usr.MustCreate()

			prog := referralprogram.New(db)
			prog.Actions = []referralprogram.Action{
				referralprogram.Action{
					Trigger: referralprogram.Trigger{
						Type: referralprogram.ReferralsGreaterThanOrEquals,
						ReferralsGreaterThanOrEqualsTrigger: referralprogram.ReferralsGreaterThanOrEqualsTrigger{
							ReferralsGreaterThanOrEquals: 2,
						},
					},
					Name: "test",
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
			prog.Actions = []referralprogram.Action{
				referralprogram.Action{
					Trigger: referralprogram.Trigger{
						Type: referralprogram.ReferralsGreaterThanOrEquals,
						ReferralsGreaterThanOrEqualsTrigger: referralprogram.ReferralsGreaterThanOrEqualsTrigger{
							ReferralsGreaterThanOrEquals: 1,
						},
					},
					Name: "test",
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

			Expect(usr.Referrers[0].State["test_done"].(bool)).To(Equal(true))

			err = usr.CalculateBalances()
			Expect(err).ToNot(HaveOccurred())
			Expect(usr.Balances[currency.USD]).To(Equal(currency.Cents(7)))
		})

		It("should work with balance triggers", func() {
			usr := user.Fake(db)
			usr.MustCreate()

			prog := referralprogram.New(db)
			prog.Actions = []referralprogram.Action{
				referralprogram.Action{
					Trigger: referralprogram.Trigger{
						Type: referralprogram.CreditGreaterThanOrEquals,
						CreditGreaterThanOrEqualsTrigger: referralprogram.CreditGreaterThanOrEqualsTrigger{
							CreditGreaterThanOrEquals: 1,
							Currency:                  currency.USD,
						},
					},
					Name: "test",
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
			prog.Actions = []referralprogram.Action{
				referralprogram.Action{
					Trigger: referralprogram.Trigger{
						Type: referralprogram.CreditGreaterThanOrEquals,
						CreditGreaterThanOrEqualsTrigger: referralprogram.CreditGreaterThanOrEqualsTrigger{
							CreditGreaterThanOrEquals: 8,
							Currency:                  currency.USD,
						},
					},
					Name: "test",
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
			prog.Actions = []referralprogram.Action{
				referralprogram.Action{
					Trigger: referralprogram.Trigger{
						Type: referralprogram.ReferralsGreaterThanOrEquals,
						CreditGreaterThanOrEqualsTrigger: referralprogram.CreditGreaterThanOrEqualsTrigger{
							CreditGreaterThanOrEquals: 7,
							Currency:                  currency.USD,
						},
					},
					Name: "test",
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

			Expect(usr.Referrers[0].State["test_done"].(bool)).To(Equal(true))

			err = usr.CalculateBalances()
			Expect(err).ToNot(HaveOccurred())
			Expect(usr.Balances[currency.USD]).To(Equal(currency.Cents(7)))
		})

		// FIt("Really?", func() {
		// 	str := "EK9E344442BI5nia9i82pdi98ip0jvqz"
		// 	arr := make([]byte, len(str))
		// 	copy(arr[:], str)
		// 	tok, err := token.FromString("eyJhbGciOiJIUzUxMiIsInR5cCI6IkpXVCJ9.eyJiaXQiOjQ1MDM2MTcwNzU2NzUxNzIsImp0aSI6ImZBcnVLbXhLUXE0Iiwic3ViIjoiNE5UeFhsUXJ0YiJ9.fOUs-H-ALpW2LtZfwT7D1sAn3Ipq7NYvnTclRZGXwRK7XvIBBovQgjB8xmezllH65LYR6hl_Wz8tr6wREJV_OQ", arr)
		// 	tok.Sec = arr
		// 	Expect(err).ToNot(HaveOccurred())
		// 	ok, err := tok.Verify(ctx, arr)
		// 	log.Error("Yay %s", tok.Secret)
		// 	log.Error("2 %s", tok.String())
		// 	Expect(err).ToNot(HaveOccurred())
		// 	Expect(ok).To(Equal(true))
		// })

		// Deprecate soon
		It("should work with old balance triggers", func() {
			usr := user.Fake(db)
			usr.MustCreate()

			prog := referralprogram.New(db)
			prog.Triggers = []int{0}
			prog.Actions = []referralprogram.Action{
				referralprogram.Action{
					Name: "test",
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

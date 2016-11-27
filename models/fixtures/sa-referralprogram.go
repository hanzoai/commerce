package fixtures

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/datastore"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/referral"
	"crowdstart.com/models/referralprogram"
	"crowdstart.com/models/types/currency"
)

var StonedReferralProgram = New("stoned-referralprogram", func(c *gin.Context) *referralprogram.ReferralProgram {
	db := datastore.New(c)

	org := organization.New(db)
	org.MustGetById("stoned")

	nsDb := datastore.New(org.Namespaced(org.Context()))

	rp := referralprogram.New(nsDb)

	rp.Name = "Stoned Referral Program"
	rp.Actions = []referralprogram.Action{
		referralprogram.Action{
			Type: referralprogram.StoreCredit,
			Name: "Sign Up Action",

			CreditAction: referralprogram.CreditAction{
				Currency: currency.PNT,
				Amount:   currency.Cents(10),
			},

			Trigger: referralprogram.Trigger{
				Event: referral.NewUser,
				Type:  referralprogram.Always,
			},
		},
		referralprogram.Action{
			Type: referralprogram.StoreCredit,
			Name: "Sale Action",

			CreditAction: referralprogram.CreditAction{
				Currency: currency.PNT,
				Amount:   currency.Cents(420),
			},

			Trigger: referralprogram.Trigger{
				Event: referral.NewOrder,
				Type:  referralprogram.Always,
			},
		},

		referralprogram.Action{
			Type: referralprogram.SendUserEmail,
			Name: "L1 Referrer",
			Once: true,

			SendTransactionalUserEmailAction: referralprogram.SendTransactionalUserEmailAction{
				EmailTemplate: "referral-level-1-reached",
			},

			Trigger: referralprogram.Trigger{
				Type: referralprogram.CreditGreaterThan,

				CreditGreaterThanTrigger: referralprogram.CreditGreaterThanTrigger{
					Currency:          currency.PNT,
					CreditGreaterThan: currency.Cents(840),
				},
			},
		},

		referralprogram.Action{
			Type: referralprogram.SendUserEmail,
			Name: "L2 Referrer",
			Once: true,

			SendTransactionalUserEmailAction: referralprogram.SendTransactionalUserEmailAction{
				EmailTemplate: "referral-level-2-reached",
			},

			Trigger: referralprogram.Trigger{
				Type: referralprogram.CreditGreaterThan,

				CreditGreaterThanTrigger: referralprogram.CreditGreaterThanTrigger{
					Currency:          currency.PNT,
					CreditGreaterThan: currency.Cents(1680),
				},
			},
		},

		referralprogram.Action{
			Type: referralprogram.SendUserEmail,
			Name: "L3 Referrer",
			Once: true,

			SendTransactionalUserEmailAction: referralprogram.SendTransactionalUserEmailAction{
				EmailTemplate: "referral-level-3-reached",
			},

			Trigger: referralprogram.Trigger{
				Type: referralprogram.CreditGreaterThan,

				CreditGreaterThanTrigger: referralprogram.CreditGreaterThanTrigger{
					Currency:          currency.PNT,
					CreditGreaterThan: currency.Cents(2520),
				},
			},
		},

		referralprogram.Action{
			Type: referralprogram.SendUserEmail,
			Name: "L4 Referrer",
			Once: true,

			SendTransactionalUserEmailAction: referralprogram.SendTransactionalUserEmailAction{
				EmailTemplate: "referral-level-4-reached",
			},

			Trigger: referralprogram.Trigger{
				Type: referralprogram.CreditGreaterThan,

				CreditGreaterThanTrigger: referralprogram.CreditGreaterThanTrigger{
					Currency:          currency.PNT,
					CreditGreaterThan: currency.Cents(3360),
				},
			},
		},

		referralprogram.Action{
			Type: referralprogram.SendUserEmail,
			Name: "L5 Referrer",
			Once: true,

			SendTransactionalUserEmailAction: referralprogram.SendTransactionalUserEmailAction{
				EmailTemplate: "referral-level-4-reached",
			},

			Trigger: referralprogram.Trigger{
				Type: referralprogram.CreditGreaterThan,

				CreditGreaterThanTrigger: referralprogram.CreditGreaterThanTrigger{
					Currency:          currency.PNT,
					CreditGreaterThan: currency.Cents(4200),
				},
			},
		},
	}

	rp.MustPut()

	return rp
})

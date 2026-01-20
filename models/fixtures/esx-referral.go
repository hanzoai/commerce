package fixtures

import (
	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/product"
	"github.com/hanzoai/commerce/models/referral"
	"github.com/hanzoai/commerce/models/referralprogram"
	"github.com/hanzoai/commerce/models/types/currency"
)

var _ = New("esx-referral", func(c *gin.Context) *organization.Organization {
	db := datastore.New(c)

	org := organization.New(db)
	org.Name = "esx"
	org.GetOrCreate("Name=", org.Name)

	nsDb := datastore.New(org.Namespaced(c))

	// Doge shirt
	prod := product.New(db)
	prod.Slug = "ticket"
	prod.GetOrCreate("Slug=", prod.Slug)
	prod.Name = "Test Ticker"
	prod.Description = `Ticket for Testing Our Checkout & Referral Program`
	prod.Price = 2000
	prod.Currency = currency.USD
	prod.MustPut()

	rp := referralprogram.New(nsDb)
	rp.Name = "ESX Referral Program"
	rp.GetOrCreate("Name=", rp.Name)

	rp.Actions = []referralprogram.Action{
		// referralprogram.Action{
		// 	Type: referralprogram.StoreCredit,
		// 	Name: "Sign Up Action",

		// 	CreditAction: referralprogram.CreditAction{
		// 		Currency: currency.PNT,
		// 		Amount:   currency.Cents(10),
		// 	},

		// 	Trigger: referralprogram.Trigger{
		// 		Event: referral.NewUser,
		// 		Type:  referralprogram.Always,
		// 	},
		// },
		referralprogram.Action{
			Type: referralprogram.StoreCredit,
			Name: "Sale Action",

			CreditAction: referralprogram.CreditAction{
				Currency: currency.PNT,
				Amount:   currency.Cents(1),
			},

			Trigger: referralprogram.Trigger{
				Event: referral.NewOrder,
				Type:  referralprogram.Always,
			},
		},
	}

	rp.MustUpdate()

	return org
})

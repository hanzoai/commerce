package fixtures

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/auth/password"
	"hanzo.io/datastore"
	"hanzo.io/models/organization"
	"hanzo.io/models/product"
	"hanzo.io/models/types/currency"
	"hanzo.io/models/referral"
	"hanzo.io/models/referralprogram"
	"hanzo.io/models/user"
)

var Halcyon = New("halcyon", func(c *gin.Context) *organization.Organization {
	db := datastore.New(c)

	org := organization.New(db)
	org.Name = "halcyon"
	org.GetOrCreate("Name=", org.Name)

	u := user.New(db)
	u.Email = "halcyon@hanzo.ai"
	u.GetOrCreate("Email=", u.Email)
	u.FirstName = "Halcyon"
	u.LastName = "User"
	u.Organizations = []string{org.Id()}
	u.PasswordHash, _ = password.Hash("Ki4TDz1ciHCNRsFs")
	u.Put()

	org.FullName = "Halcyon Bio"
	org.Owners = []string{u.Id()}
	org.Website = "http://beta.halcyon.bio"
	org.SecretKey = []byte("VwqutxegjNfz3kTo6LYMJIDQlUHxPFXHdLdiUdPdrS7v2L7fkmfn8ltqzrUmw58V")

	org.Fees.Card.Flat = 50
	org.Fees.Card.Percent = 0.05
	org.Fees.Affiliate.Flat = 0
	org.Fees.Affiliate.Percent = 0.20

	org.Email.Defaults.Enabled = true
	org.Email.Defaults.FromName = "Halcyon Bio"
	org.Email.Defaults.FromEmail = "hi@halcyon.bio"

	org.SignUpOptions.ImmediateLogin = true

	org.MustUpdate()

	nsdb := datastore.New(org.Namespaced(db.Context))

	// Create earphone product
	prod1 := product.New(nsdb)
	prod1.Slug = "earphone"
	prod1.GetOrCreate("Slug=", prod1.Slug)
	prod1.Name = "Bottle - 60 Capsules"
	prod1.Description = "30 Day Supply"
	prod1.SKU = "865524000406"
	prod1.Price = currency.Cents(8999)
	prod1.Hidden = false
	prod1.MustUpdate()

	// Create earphone product
	prod2 := product.New(nsdb)
	prod2.Slug = "60caps"
	prod2.GetOrCreate("Slug=", prod2.Slug)
	prod2.Name = "Bottle - 60 Capsules - Subscription"
	prod2.Description = "30 Day Supply"
	prod2.SKU = "865524000406-sub"
	prod2.Price = currency.Cents(6997)
	prod2.Hidden = false
	prod2.MustUpdate()

	// Create earphone product
	prod3 := product.New(nsdb)
	prod3.Slug = "sample pack"
	prod3.GetOrCreate("Slug=", prod3.Slug)
	prod3.Name = "Sample Pack"
	prod3.Description = "Try Halcyon"
	prod3.SKU = "865524000406"
	prod3.Price = currency.Cents(19999)
	prod3.Inventory = 9000
	prod3.Preorder = true
	prod3.Hidden = false
	prod3.MustUpdate()

	rp := referralprogram.New(nsdb)
	rp.Name = "Cover Referral Program"
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

	// Create earphone product
	return org
})

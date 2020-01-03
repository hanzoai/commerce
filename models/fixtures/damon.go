package fixtures

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/auth/password"
	"hanzo.io/datastore"
	"hanzo.io/models/namespace"
	"hanzo.io/models/organization"
	"hanzo.io/models/product"
	"hanzo.io/models/store"
	"hanzo.io/models/types/currency"
	"hanzo.io/models/user"
	"hanzo.io/types/email"
	"hanzo.io/types/website"
)

var _ = New("damon", func(c *gin.Context) *organization.Organization {
	db := datastore.New(c)

	org := organization.New(db)
	org.Name = "damon"
	org.GetOrCreate("Name=", org.Name)

	usr := user.New(db)
	usr.Email = "damon@hanzo.ai"
	usr.GetOrCreate("Email=", usr.Email)
	usr.FirstName = "Damon"
	usr.LastName = ""
	usr.Organizations = []string{org.Id()}
	usr.PasswordHash, _ = password.Hash("pp2Damon!zO")
	usr.MustUpdate()

	org.FullName = "Damon Inc"
	org.Owners = []string{usr.Id()}
	org.Websites = []website.Website{website.Website{Type: website.Production, Url: "https://damonmotorcycles.com/"}}
	org.SecretKey = []byte("EZ2E011iX2Bp5lv149N2STd1d580cU58")
	org.AddDefaultTokens()

	org.Fees.Card.Flat = 0
	org.Fees.Card.Percent = 0.05
	org.Fees.Affiliate.Flat = 50
	org.Fees.Affiliate.Percent = 0.30

	org.Mailchimp.APIKey = ""
	org.Mailchimp.ListId = "7849878695"

	// Email configuration
	org.Mandrill.APIKey = ""

	org.Email.Enabled = true
	org.Email.Defaults.From = email.Email{
		Name:    "Damon Motorcycles",
		Address: "hi@damonmotorcycles.com",
	}

	// org.Email.Order.Confirmation.Subject = "damon Earphones Order Confirmation"
	// org.Email.Order.Confirmation.HTML = readEmailTemplate("/resources/damon/emails/order-confirmation.html")
	// org.Email.Order.Confirmation.Enabled = true

	// Save org into default namespace
	org.MustUpdate()

	// Save namespace so we can decode keys for this organization later
	ns := namespace.New(db)
	ns.Name = org.Name
	ns.GetOrCreate("Name=", ns.Name)
	ns.IntId = org.Key().IntID()
	ns.MustUpdate()

	nsdb := datastore.New(org.Namespaced(db.Context))

	// Create default store
	stor := store.New(nsdb)
	stor.Name = "Website"
	stor.GetOrCreate("Name=", stor.Name)
	stor.MustSetKey("7RtpEPYmCnJrnB")
	stor.Prefix = "/"
	stor.Currency = currency.USD
	stor.Mailchimp.APIKey = ""
	stor.Mailchimp.ListId = "7849878695"
	stor.MustUpdate()

	// Create motorcycle product
	prod := product.New(nsdb)
	prod.Slug = "HS"
	prod.GetOrCreate("Slug=", prod.Slug)
	prod.MustSetKey("84cguxepxk")
	prod.Name = "Damon Motorcycles Hypersport HS Reservation"
	prod.Description = ""
	prod.Price = currency.Cents(10000)
	prod.Inventory = 9000
	prod.Preorder = true
	prod.Hidden = false
	prod.MustUpdate()

	premierSlugs := []string{
		"HSP-BGL",
		"HSP-BRS",
		"HSP-GGP",
		"HSP-GRS",
		"HSP-GWP",
		"HSP-RWS",
		"HSP-WRW",
		"HSP-BGP",
		"HSP-BRW",
		"HSP-GGRS",
		"HSP-GRW",
		"HSP-RWL",
		"HSP-WGL",
		"HSP-BGRS",
		"HSP-GBRS",
		"HSP-GGS",
		"HSP-GRWL",
		"HSP-RWP",
		"HSP-WRRS",
		"HSP-BGW",
		"HSP-GBW",
		"HSP-GRP",
		"HSP-GWL",
		"HSP-RWRS",
		"HSP-WRS",
	}

	for _, s := range premierSlugs {
		prod := product.New(nsdb)
		prod.Slug = s
		prod.GetOrCreate("Slug=", prod.Slug)
		prod.MustSetKey("84cguxepxk")
		prod.Name = "Damon Motorcycles Hypersport Premier " + s
		prod.Description = ""
		prod.Price = currency.Cents(100000)
		prod.Inventory = 9000
		prod.Preorder = true
		prod.Hidden = false
		prod.MustUpdate()
	}

	return org
})

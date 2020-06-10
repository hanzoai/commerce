package fixtures

import (
	// "time"
	"bytes"

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

var _ = New("karma", func(c *gin.Context) *organization.Organization {
	db := datastore.New(c)

	org := organization.New(db)
	org.Name = "karma"
	org.GetOrCreate("Name=", org.Name)

	usr := user.New(db)
	usr.Email = "karma@hanzo.ai"
	usr.GetOrCreate("Email=", usr.Email)
	usr.FirstName = "karma"
	usr.LastName = ""
	usr.Organizations = []string{org.Id()}
	usr.PasswordHash, _ = password.Hash("pp2karma!zO")
	usr.MustUpdate()

	org.FullName = "Karma Inc"
	org.Owners = []string{usr.Id()}
	org.Websites = []website.Website{website.Website{Type: website.Production, Url: "https://karmabikinis.online"}}
	org.EmailWhitelist = "*.hanzo.ai *.karmabikinis.online"
	if bytes.Compare(org.SecretKey, []byte("1gML2pOHK4PW8xMc")) != 0 {
		org.SecretKey = []byte("1gML2pOHK4PW8xMc")
		org.AddDefaultTokens()
	}

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
		Name:    "Karma",
		Address: "hi@karmabikinis.online",
	}

	// org.Email.Order.Confirmation.Subject = "karma Earphones Order Confirmation"
	// org.Email.Order.Confirmation.HTML = readEmailTemplate("/resources/karma/emails/order-confirmation.html")
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
	stor.Prefix = "/"
	stor.Currency = currency.USD
	stor.Mailchimp.APIKey = ""
	stor.Mailchimp.ListId = "7849878695"
	stor.MustUpdate()

	{
		prod := product.New(nsdb)
		prod.Slug = "masks"
		prod.GetOrCreate("Slug=", prod.Slug)
		prod.Name = "2x Masks"
		prod.Description = ""
		prod.Price = currency.Cents(5000)
		prod.ListPrice = currency.Cents(5000)
		prod.Inventory = 9000
		prod.Preorder = true
		prod.MustUpdate()
	}

	{
		prod := product.New(nsdb)
		prod.Slug = "karma-swim"
		prod.GetOrCreate("Slug=", prod.Slug)
		prod.Name = "Karma Swim"
		prod.Description = ""
		prod.Price = currency.Cents(20000)
		prod.ListPrice = currency.Cents(20000)
		prod.Inventory = 9000
		prod.Preorder = true
		prod.MustUpdate()
	}

	{
		prod := product.New(nsdb)
		prod.Slug = "credit"
		prod.GetOrCreate("Slug=", prod.Slug)
		prod.Name = "2x Credit"
		prod.Description = ""
		prod.Price = currency.Cents(50000)
		prod.ListPrice = currency.Cents(50000)
		prod.Inventory = 9000
		prod.Preorder = true
		prod.MustUpdate()
	}

	{
		prod := product.New(nsdb)
		prod.Slug = "custom-designs"
		prod.GetOrCreate("Slug=", prod.Slug)
		prod.Name = "Custom Designs"
		prod.Description = ""
		prod.Price = currency.Cents(100000)
		prod.ListPrice = currency.Cents(100000)
		prod.Inventory = 9000
		prod.Preorder = true
		prod.MustUpdate()
	}

	{
		prod := product.New(nsdb)
		prod.Slug = "sponsor"
		prod.GetOrCreate("Slug=", prod.Slug)
		prod.Name = "Sponsor a Karma Shoot"
		prod.Description = ""
		prod.Price = currency.Cents(500000)
		prod.ListPrice = currency.Cents(500000)
		prod.Inventory = 9000
		prod.Preorder = true
		prod.MustUpdate()
	}

	{
		prod := product.New(nsdb)
		prod.Slug = "capsule-collection"
		prod.GetOrCreate("Slug=", prod.Slug)
		prod.Name = "Capsule Collection"
		prod.Description = ""
		prod.Price = currency.Cents(1500000)
		prod.ListPrice = currency.Cents(1500000)
		prod.Inventory = 9000
		prod.Preorder = true
		prod.MustUpdate()
	}

	{
		prod := product.New(nsdb)
		prod.Slug = "design-partner"
		prod.GetOrCreate("Slug=", prod.Slug)
		prod.Name = "Design Partner"
		prod.Description = ""
		prod.Price = currency.Cents(4000000)
		prod.ListPrice = currency.Cents(4000000)
		prod.Inventory = 9000
		prod.Preorder = true
		prod.MustUpdate()
	}

	{
		prod := product.New(nsdb)
		prod.Slug = "travel"
		prod.GetOrCreate("Slug=", prod.Slug)
		prod.Name = "Travel the World w/ Karma"
		prod.Description = ""
		prod.Price = currency.Cents(10000000)
		prod.ListPrice = currency.Cents(10000000)
		prod.Inventory = 9000
		prod.Preorder = true
		prod.MustUpdate()
	}

	return org
})

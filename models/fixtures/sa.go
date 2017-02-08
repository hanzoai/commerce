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
)

var Stoned = New("stoned", func(c *gin.Context) *organization.Organization {
	db := datastore.New(c)

	org := organization.New(db)
	org.Name = "stoned"
	org.GetOrCreate("Name=", org.Name)
	org.MustSetKey("4NTxXlQrtb")

	usr := user.New(db)
	usr.Email = "dev@hanzo.ai"
	usr.GetOrCreate("Email=", usr.Email)
	usr.FirstName = "Founders"
	usr.LastName = ""
	usr.Organizations = []string{org.Id()}
	usr.PasswordHash, _ = password.Hash("1Stoned23")
	usr.MustUpdate()

	org.FullName = "Stoned, LLC"
	org.Owners = []string{usr.Id()}
	org.Website = "http://www.stoned.audio"
	org.SecretKey = []byte("EK9E344442BI5nia9i82pdi98ip0jvqz")
	org.AddDefaultTokens()

	org.Fees.Card.Flat = 50
	org.Fees.Card.Percent = 0.05
	org.Fees.Affiliate.Flat = 30
	org.Fees.Affiliate.Percent = 0.30

	org.Mailchimp.APIKey = ""
	org.Mailchimp.ListId = "421751eb03"

	// Email configuration
	org.Mandrill.APIKey = ""

	org.Email.Defaults.Enabled = true
	org.Email.Defaults.FromName = "Stoned Audio"
	org.Email.Defaults.FromEmail = "dev@hanzo.ai"

	//org.Email.OrderConfirmation.Subject = "Stoned Audio Order Confirmation"
	//org.Email.OrderConfirmation.Template = readEmailTemplate("/resources/sa/emails/order-confirmation.html")
	//org.Email.OrderConfirmation.Enabled = true

	//org.Email.User.PasswordReset.Template = readEmailTemplate("/resources/sa/emails/user-password-reset.html")
	//org.Email.User.PasswordReset.Subject = "Reset your password"
	//org.Email.User.PasswordReset.Enabled = true

	//org.Email.User.EmailConfirmation.Template = readEmailTemplate("/resources/sa/emails/user-email-confirmation.html")
	//org.Email.User.EmailConfirmation.Subject = "Please confirm your email"
	//org.Email.User.EmailConfirmation.Enabled = true

	//org.Email.User.EmailConfirmed.Subject = "Thank you for confirming your email"
	//org.Email.User.EmailConfirmed.Template = readEmailTemplate("/resources/sa/emails/user-email-confirmed.html")
	//org.Email.User.EmailConfirmed.Enabled = false

	org.Stripe.AccessToken = ""
	org.Stripe.Live.AccessToken = ""
	org.Stripe.Live.Livemode = true
	org.Stripe.Live.PublishableKey = "pk_live_HYt7tGsPrtvKKDCjH9zYQ8KG"
	org.Stripe.Live.RefreshToken = ""
	org.Stripe.Live.Scope = "read_write"
	org.Stripe.Live.TokenType = "bearer"
	org.Stripe.Live.UserId = "acct_1978ZsDEqW0iccHt"

	org.Stripe.Test.AccessToken = ""
	org.Stripe.Test.Livemode = false
	org.Stripe.Test.PublishableKey = "pk_test_vIu4eBlMDi6HlylbfzNFEst7"
	org.Stripe.Test.RefreshToken = ""
	org.Stripe.Test.Scope = "read_write"
	org.Stripe.Test.TokenType = "bearer"
	org.Stripe.Test.UserId = "acct_1978ZsDEqW0iccHt"

	org.Stripe.PublishableKey = "pk_live_HYt7tGsPrtvKKDCjH9zYQ8KG"
	org.Stripe.RefreshToken = ""
	org.Stripe.UserId = "acct_1978ZsDEqW0iccHt"

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
	stor.Name = "development"
	stor.GetOrCreate("Name=", stor.Name)
	stor.MustSetKey("KawdtZuoMY")
	stor.Prefix = "/"
	stor.Currency = currency.USD
	stor.Mailchimp.APIKey = ""
	stor.Mailchimp.ListId = "421751eb03"
	stor.MustUpdate()

	// Create earphone product
	prod := product.New(nsdb)
	prod.Slug = "earphone"
	prod.GetOrCreate("Slug=", prod.Slug)
	prod.MustSetKey("MrbcmBZbsd")
	prod.Name = "Stoned Earphones"
	prod.Description = "2 Ear Buds, 1 Charging Case"
	prod.Price = currency.Cents(9999)
	prod.Inventory = 9000
	prod.Preorder = true
	prod.Hidden = false
	prod.MustUpdate()

	return org
})

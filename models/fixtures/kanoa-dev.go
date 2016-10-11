package fixtures

import (
	"time"

	"github.com/gin-gonic/gin"

	"crowdstart.com/auth/password"
	"crowdstart.com/datastore"
	"crowdstart.com/models/namespace"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/product"
	"crowdstart.com/models/store"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/models/user"
	"crowdstart.com/util/token"
)

var _ = New("kanoa-dev", func(c *gin.Context) *organization.Organization {
	db := datastore.New(c)

	// Create user
	usr := user.New(db)
	usr.Email = "cival@getkanoa.com"
	usr.GetOrCreate("Email=", usr.Email)
	usr.FirstName = "Cival"
	usr.LastName = ""
	usr.PasswordHash, _ = password.Hash("1Kanoa23")

	// Create organization
	org := organization.New(db)
	org.Name = "kanoa"
	org.GetOrCreate("Name=", org.Name)
	org.SetKey("vMAXTXuKa3")

	// Set organization on user
	usr.Organizations = []string{org.Id()}

	org.FullName = "KANOA Inc"
	org.Owners = []string{usr.Id()}
	org.Website = "https://www.getkanoa.com"
	org.SecretKey = []byte("EZ2E011iX2Bp5lv149N2STd1d580cU58")

	org.Fees.Card.Flat = 50
	org.Fees.Card.Percent = 0.05
	org.Fees.Affiliate.Flat = 30
	org.Fees.Affiliate.Percent = 0.30

	// Integration configuration
	org.Mailchimp.APIKey = ""
	org.Mailchimp.ListId = "23ad4e4ba4"
	org.Mandrill.APIKey = ""

	// Affiliate configuration
	org.Affiliate.SuccessUrl = "http://localhost:1987/account/"
	org.Affiliate.ErrorUrl = "http://localhost:1987/account/"

	// Paypal Config
	org.Paypal.ConfirmUrl = "https://www.getkanoa.com"
	org.Paypal.CancelUrl = "https://www.getkanoa.com"

	org.Paypal.Live.Email = "cival@getkanoa.com"
	org.Paypal.Live.SecurityUserId = "cival_api1.getkanoa.com"
	org.Paypal.Live.ApplicationId = "APP-6PG93936C8597944N"
	org.Paypal.Live.SecurityPassword = "2YNUBS9TB9U7EDCM"
	org.Paypal.Live.SecuritySignature = "AFcWxV21C7fd0v3bYYYRCpSSRl31AZ6CAELso7zxPQz8gLc5YSsz6Iza"

	org.Paypal.Test.Email = "cival-facilitator@getkanoa.com"
	org.Paypal.Test.SecurityUserId = "cival-facilitator_api1.getkanoa.com"
	org.Paypal.Test.ApplicationId = "APP-80W284485P519543T"
	org.Paypal.Test.SecurityPassword = "XMDRP9CF75ESA8P8"
	org.Paypal.Test.SecuritySignature = "AcoBndPxINN2yEkgSKALAXYErpWTAFpUk3S6BucWeefHiUNpGxIleLof"

	// Email config
	org.Email.Defaults.Enabled = true
	org.Email.Defaults.FromName = "KANOA"
	org.Email.Defaults.FromEmail = "hi@kanoa.com"

	org.Email.OrderConfirmation.Subject = "KANOA Earphones Order Confirmation"
	org.Email.OrderConfirmation.Template = readEmailTemplate("/resources/kanoa/emails/order-confirmation.html")
	org.Email.OrderConfirmation.Enabled = true

	org.Email.User.PasswordReset.Template = readEmailTemplate("/resources/kanoa/emails/user-password-reset.html")
	org.Email.User.PasswordReset.Subject = "Reset your KANOA password"
	org.Email.User.PasswordReset.Enabled = true

	org.Email.User.EmailConfirmation.Template = readEmailTemplate("/resources/kanoa/emails/user-email-confirmation.html")
	org.Email.User.EmailConfirmation.Subject = "Please confirm your email"
	org.Email.User.EmailConfirmation.Enabled = true

	org.Email.User.EmailConfirmed.Subject = "Thank you for confirming your email"
	org.Email.User.EmailConfirmed.Template = readEmailTemplate("/resources/kanoa/emails/user-email-confirmed.html")
	org.Email.User.EmailConfirmed.Enabled = false

	// Stripe tokens
	org.Stripe.AccessToken = "sk_test_aqA1nQ6aWNjJoIaynPIwdY0w"
	org.Stripe.Live.AccessToken = "sk_test_aqA1nQ6aWNjJoIaynPIwdY0w"
	org.Stripe.Live.PublishableKey = "pk_test_OhE3VKqrWXxht14ztjgluGgG"
	org.Stripe.Live.RefreshToken = "rt_6tPyHWMqDd3C2Ii5IX85lzCqHDN5msJGg1n6zNQgBKdQZONv"
	org.Stripe.Live.Scope = "read_write"
	org.Stripe.Live.UserId = "acct_16PFH2Iau5NyccPf"
	org.Stripe.PublishableKey = "pk_test_OhE3VKqrWXxht14ztjgluGgG"
	org.Stripe.RefreshToken = "rt_6tPyHWMqDd3C2Ii5IX85lzCqHDN5msJGg1n6zNQgBKdQZONv"
	org.Stripe.Test.AccessToken = "sk_test_aqA1nQ6aWNjJoIaynPIwdY0w"
	org.Stripe.Test.PublishableKey = "pk_test_OhE3VKqrWXxht14ztjgluGgG"
	org.Stripe.Test.RefreshToken = "rt_6tPyHWMqDd3C2Ii5IX85lzCqHDN5msJGg1n6zNQgBKdQZONv"
	org.Stripe.Test.Scope = "read_write"
	org.Stripe.Test.UserId = "acct_16PFH2Iau5NyccPf"
	org.Stripe.UserId = "acct_16PFH2Iau5NyccPf"

	// API Tokens
	org.Tokens = []token.Token{
		token.Token{
			EntityId:    "vMAXTXuKa3",
			Id:          "OUmLMjm",
			IssuedAt:    time.Now(),
			Name:        "live-secret-key",
			Permissions: 20,
			Secret:      []byte("EZ2E011iX2Bp5lv149N2STd1d580cU58"),
		},
		token.Token{
			EntityId:    "vMAXTXuKa3",
			Id:          "lpmxHnNMN8Y",
			IssuedAt:    time.Now(),
			Name:        "live-published-key",
			Permissions: 4503617075675172,
			Secret:      []byte("EZ2E011iX2Bp5lv149N2STd1d580cU58"),
		},
		token.Token{
			EntityId:    "vMAXTXuKa3",
			Id:          "YYoUAGes",
			IssuedAt:    time.Now(),
			Name:        "test-secret-key",
			Permissions: 24,
			Secret:      []byte("EZ2E011iX2Bp5lv149N2STd1d580cU58"),
		},
		token.Token{
			EntityId:    "vMAXTXuKa3",
			Id:          "WSXQDoVe6Bs",
			IssuedAt:    time.Now(),
			Name:        "test-published-key",
			Permissions: 4503617075675176,
			Secret:      []byte("EZ2E011iX2Bp5lv149N2STd1d580cU58"),
		},
	}

	// Save namespace so we can decode keys for this organization later
	ns := namespace.New(db)
	ns.Name = org.Name
	ns.GetOrCreate("Name=", ns.Name)
	ns.IntId = org.Key().IntID()
	ns.Update()

	// Create namespaced context
	nsdb := datastore.New(org.Namespaced(db.Context))

	// Create new store
	stor := store.New(nsdb)
	stor.Name = "development"
	stor.GetOrCreate("Name=", stor.Name)
	stor.SetKey("MZbtooKHjM")
	stor.Prefix = "/"
	stor.Currency = currency.USD
	stor.Mailchimp.APIKey = ""
	stor.Mailchimp.ListId = "23ad4e4ba4"

	// Set default store on org
	org.DefaultStore = stor.Id()

	// Fetch earphones
	prod := product.New(nsdb)
	prod.Slug = "earphone"
	prod.GetOrCreate("Slug=", prod.Slug)
	prod.SetKey("9V84cGS9VK")
	prod.Name = "KANOA Earphone"
	prod.Description = "2 Ear Buds, 1 Charging Case, 3 Ergonomic Ear Tips, 1 Micro USB Cable"
	prod.Price = currency.Cents(19999)
	prod.Inventory = 9000
	prod.Preorder = true
	prod.Hidden = false

	// Save entities
	usr.MustUpdate()
	org.MustUpdate()
	stor.MustUpdate()
	prod.MustUpdate()

	// Create corresponding Mailchimp entities
	// client := mailchimp.New(db.Context, org.Mailchimp.APIKey)
	// client.CreateStore(stor)
	// client.CreateProduct(stor.Id(), prod)

	return org
})

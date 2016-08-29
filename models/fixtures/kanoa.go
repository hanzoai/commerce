package fixtures

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/auth/password"
	"crowdstart.com/datastore"
	"crowdstart.com/models/namespace"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/product"
	"crowdstart.com/models/store"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/models/user"
	"crowdstart.com/util/log"
)

var Kanoa = New("kanoa", func(c *gin.Context) *organization.Organization {
	db := datastore.New(c)

	org := organization.New(db)
	org.Name = "kanoa"
	org.GetOrCreate("Name=", org.Name)

	u := user.New(db)
	u.Email = "cival@getkanoa.com"
	u.GetOrCreate("Email=", u.Email)
	u.FirstName = "Cival"
	u.LastName = ""
	u.Organizations = []string{org.Id()}
	u.PasswordHash, _ = password.Hash("1Kanoa23")
	u.Update()

	org.FullName = "KANOA Inc"
	org.Owners = []string{u.Id()}
	org.Website = "http://www.getkanoa.com"
	org.SecretKey = []byte("EZ2E011iX2Bp5lv149N2STd1d580cU58")
	org.AddDefaultTokens()
	org.Fee = 0.05
	org.Mailchimp.APIKey = ""
	org.Mailchimp.ListId = "23ad4e4ba4"

	// Email configuration
	org.Mandrill.APIKey = ""

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

	// Save org into default namespace
	org.Put()

	// Save namespace so we can decode keys for this organization later
	ns := namespace.New(db)
	ns.Name = org.Name
	ns.IntId = org.Key().IntID()
	err := ns.Put()
	if err != nil {
		log.Warn("Failed to put namespace: %v", err)
	}

	nsdb := datastore.New(org.Namespaced(db.Context))

	// Create default store
	stor := store.New(nsdb)
	stor.Name = "default"
	stor.GetOrCreate("Name=", stor.Name)
	stor.Prefix = "/"
	stor.Currency = currency.USD
	stor.Mailchimp.APIKey = ""
	stor.Mailchimp.ListId = "23ad4e4ba4"
	stor.Update()

	// Create earphone product
	prod := product.New(nsdb)
	prod.Slug = "earphone"
	prod.GetOrCreate("Slug=", prod.Slug)
	prod.Name = "KANOA Earphone"
	prod.Description = "2 Ear Buds, 1 Charging Case, 3 Ergonomic Ear Tips, 1 Micro USB Cable"
	prod.Price = currency.Cents(19999)
	prod.Inventory = 9000
	prod.Preorder = true
	prod.Hidden = false
	prod.Update()

	return org
})

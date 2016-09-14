package fixtures

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/auth/password"
	"crowdstart.com/datastore"
	"crowdstart.com/models/namespace"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/user"
	"crowdstart.com/util/log"
)

var Ludela = New("ludela", func(c *gin.Context) *organization.Organization {
	db := datastore.New(c)

	org := organization.New(db)
	org.Name = "ludela"
	org.GetOrCreate("Name=", org.Name)

	u := user.New(db)
	u.Email = "jamie@ludela.com"
	u.GetOrCreate("Email=", u.Email)
	u.FirstName = "Jamie"
	u.LastName = ""
	u.Organizations = []string{org.Id()}
	u.PasswordHash, _ = password.Hash("1Ludela23")
	u.Put()

	org.FullName = "Ludela Inc"
	org.Owners = []string{u.Id()}
	org.Website = "http://www.ludela.com"
	org.SecretKey = []byte("EU8E011iX2Bp5lv481N2STd1d999cU58")
	org.AddDefaultTokens()

	org.Fees.Card.Flat = 50
	org.Fees.Card.Percent = 0.05
	org.Fees.Affiliate.Flat = 30
	org.Fees.Affiliate.Percent = 0.30

	// Email configuration
	org.Mandrill.APIKey = "40gP4DdLRLHo1QX_A8mfHw"

	org.Email.Defaults.Enabled = true
	org.Email.Defaults.FromName = "Ludela"
	org.Email.Defaults.FromEmail = "hi@ludela.com"

	// org.Email.OrderConfirmation.Subject = "KANOA Earphones Order Confirmation"
	// org.Email.OrderConfirmation.Template = readEmailTemplate("/resources/kanoa/emails/order-confirmation.html")
	// org.Email.OrderConfirmation.Enabled = true

	// org.Email.User.PasswordReset.Template = readEmailTemplate("/resources/kanoa/emails/user-password-reset.html")
	// org.Email.User.PasswordReset.Subject = "Reset your KANOA password"
	// org.Email.User.PasswordReset.Enabled = true

	// org.Email.User.EmailConfirmation.Template = readEmailTemplate("/resources/kanoa/emails/user-email-confirmation.html")
	// org.Email.User.EmailConfirmation.Subject = "Please confirm your email"
	// org.Email.User.EmailConfirmation.Enabled = true

	// org.Email.User.EmailConfirmed.Subject = "Thank you for confirming your email"
	// org.Email.User.EmailConfirmed.Template = readEmailTemplate("/resources/kanoa/emails/user-email-confirmed.html")
	// org.Email.User.EmailConfirmed.Enabled = false

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

	return org
})

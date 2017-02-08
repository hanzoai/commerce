package fixtures

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/auth/password"
	"hanzo.io/datastore"
	"hanzo.io/models/namespace"
	"hanzo.io/models/organization"
	"hanzo.io/models/user"
	"hanzo.io/util/log"
)

var Crowdstart = New("crowdstart", func(c *gin.Context) *organization.Organization {
	db := datastore.New(c)

	// Create organization
	org := organization.New(db)
	org.Name = "crowdstart"
	org.GetOrCreate("Name=", org.Name)

	// Create admin
	u := user.New(db)
	u.Email = "crowdstart@verus.io"
	u.GetOrCreate("Email=", u.Email)

	// Configure org
	org.FullName = "crowdstart"
	org.Owners = []string{u.Id()}
	org.Website = "http://hanzo.io"
	org.SecretKey = []byte("zW85MZHMklGJE3hNgC5j1cxFpQ04zLb6")
	org.AddDefaultTokens()

	// Configure user
	u.FirstName = "Michael"
	u.LastName = "Walker"
	u.Organizations = []string{org.Id()}
	u.PasswordHash, _ = password.Hash("crowdstartpassword!")
	u.Put()

	// Email configuration
	// org.Mandrill.APIKey = ""

	// org.Email.Defaults.Enabled = true
	// org.Email.Defaults.FromName = "Bellabeat"
	// org.Email.Defaults.FromEmail = "hi@bellabeat.com"

	// org.Email.OrderConfirmation.Subject = "LEAF Order Confirmation"
	// org.Email.OrderConfirmation.Template = readEmailTemplate("/resources/bellabeat/emails/order-confirmation.html")
	// org.Email.OrderConfirmation.Enabled = true

	// org.Email.User.PasswordReset.Template = readEmailTemplate("/resources/bellabeat/emails/user-password-reset.html")
	// org.Email.User.PasswordReset.Subject = "Reset your Bellabeat password"
	// org.Email.User.PasswordReset.Enabled = true

	// org.Email.User.EmailConfirmation.Template = readEmailTemplate("/resources/bellabeat/emails/user-email-confirmation.html")
	// org.Email.User.EmailConfirmation.Subject = "Please confirm your email"
	// org.Email.User.EmailConfirmation.Enabled = true

	// org.Email.User.EmailConfirmed.Subject = "Thank you for confirming your email"
	// org.Email.User.EmailConfirmed.Template = readEmailTemplate("/resources/bellabeat/emails/user-email-confirmed.html")
	// org.Email.User.EmailConfirmed.Enabled = true

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

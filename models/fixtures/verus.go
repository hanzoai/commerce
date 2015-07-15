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

var Verus = New("verus", func(c *gin.Context) *organization.Organization {
	db := datastore.New(c)

	// Create organization
	org := organization.New(db)
	org.Name = "verus"
	org.GetOrCreate("Name=", org.Name)

	// Create admins
	u := user.New(db)
	u.Email = "dev@hanzo.ai"
	u.GetOrCreate("Email=", u.Email)

	u2 := user.New(db)
	u2.Email = "dev@hanzo.ai"
	u2.GetOrCreate("Email=", u2.Email)

	u3 := user.New(db)
	u3.Email = "dev@hanzo.ai"
	u3.GetOrCreate("Email=", u3.Email)

	// Configure org
	org.FullName = "verus"
	org.Owners = []string{u.Id(), u2.Id(), u3.Id()}
	org.Website = "http://www.verus.com"
	org.SecretKey = []byte("zW85MZHMklGJE3hNgC5j1cxFpQ04zLb6")
	org.AddDefaultTokens()

	// Configure user
	u.FirstName = "Michael"
	u.LastName = "Walker"
	u.Organizations = []string{org.Id()}
	u.PasswordHash, _ = password.Hash("veruspassword!")
	u.Put()

	// Configure user
	u2.FirstName = "Zach"
	u2.LastName = "Kelling"
	u2.Organizations = []string{org.Id()}
	u2.PasswordHash, _ = password.Hash("veruspassword!")
	u2.Put()

	// Configure user
	u3.FirstName = "David"
	u3.LastName = "Tai"
	u3.Organizations = []string{org.Id()}
	u3.PasswordHash, _ = password.Hash("veruspassword!")
	u3.Put()

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

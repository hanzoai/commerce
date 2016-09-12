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
	u.FirstName = "Michael"
	u.LastName = "Walker"
	u.Organizations = []string{org.Id()}
	u.PasswordHash, _ = password.Hash("veruspassword!")
	u.Put()

	u2 := user.New(db)
	u2.Email = "dev@hanzo.ai"
	u2.GetOrCreate("Email=", u2.Email)
	u2.FirstName = "Zach"
	u2.LastName = "Kelling"
	u2.Organizations = []string{org.Id()}
	u2.PasswordHash, _ = password.Hash("veruspassword!")
	u2.Put()

	u3 := user.New(db)
	u3.Email = "dev@hanzo.ai"
	u3.GetOrCreate("Email=", u3.Email)
	u3.FirstName = "David"
	u3.LastName = "Tai"
	u3.Organizations = []string{org.Id()}
	u3.PasswordHash, _ = password.Hash("veruspassword!")
	u3.Put()

	u4 := user.New(db)
	u4.Email = "tmesser@verus.io"
	u4.GetOrCreate("Email=", u4.Email)
	u4.FirstName = "Tim"
	u4.LastName = "Messer"
	u4.Organizations = []string{org.Id()}
	u4.PasswordHash, _ = password.Hash("veruspassword!")
	u4.Put()

	// u5 := user.New(db)
	// u5.Email = "dev@hanzo.ai"
	// u5.GetOrCreate("Email=", u4.Email)
	// u5.FirstName = "Marvel"
	// u5.LastName = "Mathew"
	// u5.Organizations = []string{org.Id()}
	// u5.PasswordHash, _ = password.Hash("veruspassword!")
	// u5.Put()

	// u6 := user.New(db)
	// u6.Email = "helpfulhuman@verus.io"
	// u6.GetOrCreate("Email=", u6.Email)
	// u6.FirstName = "Helpful Human"
	// u6.LastName = ""
	// u6.Organizations = []string{org.Id()}
	// u6.PasswordHash, _ = password.Hash("HelpfulHumans!")
	// u6.Put()

	// Configure org
	org.FullName = "verus"
	org.Owners = []string{u.Id(), u2.Id(), u3.Id(), u4.Id()}
	org.Website = "http://www.verus.com"
	org.SecretKey = []byte("zW85MZHMklGJE3hNgC5j1cxFpQ04zLb6")
	org.AddDefaultTokens()

	// Email configuration
	org.Mandrill.APIKey = ""

	org.Email.Defaults.Enabled = true
	org.Email.Defaults.FromName = "Bellabeat"
	org.Email.Defaults.FromEmail = "hi@bellabeat.com"

	org.Email.OrderConfirmation.Subject = "LEAF Order Confirmation"
	org.Email.OrderConfirmation.Template = readEmailTemplate("/resources/bellabeat/emails/order-confirmation.html")
	org.Email.OrderConfirmation.Enabled = true

	org.Email.User.PasswordReset.Template = readEmailTemplate("/resources/bellabeat/emails/user-password-reset.html")
	org.Email.User.PasswordReset.Subject = "Reset your Bellabeat password"
	org.Email.User.PasswordReset.Enabled = true

	org.Email.User.EmailConfirmation.Template = readEmailTemplate("/resources/bellabeat/emails/user-email-confirmation.html")
	org.Email.User.EmailConfirmation.Subject = "Please confirm your email"
	org.Email.User.EmailConfirmation.Enabled = true

	org.Email.User.EmailConfirmed.Subject = "Thank you for confirming your email"
	org.Email.User.EmailConfirmed.Template = readEmailTemplate("/resources/bellabeat/emails/user-email-confirmed.html")
	org.Email.User.EmailConfirmed.Enabled = true

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

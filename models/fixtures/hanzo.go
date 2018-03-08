package fixtures

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/config"
	"hanzo.io/datastore"
	"hanzo.io/log"
	"hanzo.io/models/organization"
	"hanzo.io/models/user"
)

var Hanzo = New("hanzo", func(c *gin.Context) *organization.Organization {
	db := datastore.New(c)

	// Create organization
	org := organization.New(db)
	org.Name = "hanzo"
	org.GetOrCreate("Name=", org.Name)

	// Create admins
	u := user.New(db)
	u.Email = "dev@hanzo.ai"
	u.GetOrCreate("Email=", u.Email)
	u.AddOrganization(org.Id())
	u.Put()

	// Configure org
	org.FullName = "hanzo"
	org.AddOwner(u.Id())
	org.Website = "https://hanzo.ai"
	org.SecretKey = []byte("95j23am4EvU2LHiFHE2gNfC31cwoP0z5")

	// Email configuration
	org.Mandrill.APIKey = config.Mandrill.APIKey

	// org.Email.Defaults.Enabled = true
	// org.Email.Defaults.FromName = "hanzo"
	// org.Email.Defaults.FromEmail = "hi@hanzo.com"

	// org.Email.OrderConfirmation.Subject = "LEAF Order Confirmation"
	// org.Email.OrderConfirmation.Template = readEmailTemplate("/resources/hanzo/emails/order-confirmation.html")
	// org.Email.OrderConfirmation.Enabled = true

	// org.Email.User.PasswordReset.Template = readEmailTemplate("/resources/hanzo/emails/user-password-reset.html")
	// org.Email.User.PasswordReset.Subject = "Reset your hanzo password"
	// org.Email.User.PasswordReset.Enabled = true

	// org.Email.User.EmailConfirmation.Template = readEmailTemplate("/resources/hanzo/emails/user-email-confirmation.html")
	// org.Email.User.EmailConfirmation.Subject = "Please confirm your email"
	// org.Email.User.EmailConfirmation.Enabled = true

	// org.Email.User.EmailConfirmed.Subject = "Thank you for confirming your email"
	// org.Email.User.EmailConfirmed.Template = readEmailTemplate("/resources/hanzo/emails/user-email-confirmed.html")
	// org.Email.User.EmailConfirmed.Enabled = true

	// Save org into default namespace
	org.Create()

	return org
})

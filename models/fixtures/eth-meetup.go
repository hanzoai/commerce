package fixtures

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/auth/password"
	"hanzo.io/datastore"
	"hanzo.io/models/organization"
	"hanzo.io/models/user"
)

var EthMeetup = New("ethmeetup", func(c *gin.Context) *organization.Organization {
	db := datastore.New(c)

	org := organization.New(db)
	org.Name = "ethmeetup"
	org.GetOrCreate("Name=", org.Name)

	u := user.New(db)
	u.Email = "david@hanzo.ai"
	u.GetOrCreate("Email=", u.Email)
	u.FirstName = "David"
	u.LastName = "Tai"
	u.Organizations = []string{org.Id()}
	u.PasswordHash, _ = password.Hash("Xtr3Lk7R")
	u.Put()

	org.FullName = "Ethereum Meetup"
	org.Owners = []string{u.Id()}
	org.Website = "http://www.ethmeetupcase.com"
	org.SecretKey = []byte("EGtFY6kqvTuMHsuSW6Qk5NduE22Xk39S")

	org.Fees.Card.Flat = 50
	org.Fees.Card.Percent = 0.05
	org.Fees.Affiliate.Flat = 30
	org.Fees.Affiliate.Percent = 0.30

	// Email configuration
	// org.Mandrill.APIKey = ""

	org.Email.Defaults.Enabled = true
	org.Email.Defaults.FromName = "Ethereum Meetup"
	org.Email.Defaults.FromEmail = "hi@ethmeetupcase.com"

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
	org.Update()

	return org
})

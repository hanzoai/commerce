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
	u.PasswordHash, _ = password.Hash("kanoapassword!")
	u.Put()

	org.FullName = "KANOA Inc"
	org.Owners = []string{u.Id()}
	org.Website = "http://www.getkanoa.com"
	org.SecretKey = []byte("EZ2E011iX2Bp5lv149N2STd1d580cU58")
	//org.AddDefaultTokens()
	org.Fee = 0.05

	// Email configuration
	org.Mandrill.APIKey = ""

	org.Paypal.SecurityUserId = "sandboxpaypal_api1.verus.io"
	org.Paypal.ApplicationId = "APP-80W284485P519543T"
	org.Paypal.SecurityPassword = "LTCEUG8Z6RZDCSWL"
	org.Paypal.SecuritySignature = "A-qfk86fpHB4QlDDX.QRap2Q4iHGAa9QjVSDBGxcNT08r.2od2UXoCdn"

	org.Paypal.TestSecurityUserId = "sandboxpaypal_api1.verus.io"
	org.Paypal.TestApplicationId = "APP-80W284485P519543T"
	org.Paypal.TestSecurityPassword = "LTCEUG8Z6RZDCSWL"
	org.Paypal.TestSecuritySignature = "A-qfk86fpHB4QlDDX.QRap2Q4iHGAa9QjVSDBGxcNT08r.2od2UXoCdn"

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

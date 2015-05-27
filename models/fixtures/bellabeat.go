package fixtures

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/auth/password"
	"crowdstart.com/config"
	"crowdstart.com/datastore"
	"crowdstart.com/models/namespace"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/user"

	"crowdstart.com/util/fs"
	"crowdstart.com/util/log"
)

var Bellabeat = New("bellabeat", func(c *gin.Context) *organization.Organization {
	db := datastore.New(c)

	org := organization.New(db)
	org.Name = "bellabeat"
	org.GetOrCreate("Name=", org.Name)

	u := user.New(db)
	u.Email = "sandro@bellabeat.com"
	u.GetOrCreate("Email=", u.Email)
	u.FirstName = "Sandro"
	u.LastName = "Mur"
	u.Organizations = []string{org.Id()}
	u.PasswordHash, _ = password.Hash("bellabeatpassword!")
	u.Put()

	u2 := user.New(db)
	u2.Email = "marko@bellabeat.com"
	u2.GetOrCreate("Email=", u.Email)
	u2.FirstName = "Marko"
	u2.LastName = "Bozic"
	u2.Organizations = []string{org.Id()}
	u2.PasswordHash, _ = password.Hash("bellabeatpassword!")
	u2.Put()

	org.FullName = "bellabeat"
	org.Owners = []string{u.Id()}
	org.Admins = []string{u2.Id()}
	org.Website = "http://www.bellabeat.com"
	org.SecretKey = []byte("yW83JZGLjkGJE2gMfB4i0bwEoP03yJa5")
	// org.AddDefaultTokens()

	// Email configuration
	org.Mandrill.APIKey = ""

	org.Email.Enabled = true
	org.Email.FromName = "Bellabeat"
	org.Email.FromEmail = "hi@bellabeat.com"
	org.Email.OrderConfirmation.Enabled = true
	org.Email.OrderConfirmation.Subject = "LEAF Order Confirmation"
	org.Email.OrderConfirmation.Template = string(fs.ReadFile(config.WorkingDir + "/resources/bellabeat/email-order-confirmation.html"))

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

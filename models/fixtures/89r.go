package fixtures

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/auth/password"
	"hanzo.io/datastore"
	"hanzo.io/models/namespace"
	"hanzo.io/models/organization"
	"hanzo.io/models/user"
	"hanzo.io/types/website"
	"hanzo.io/log"
)

var EightyNineRobotics = New("89r", func(c *gin.Context) *organization.Organization {
	db := datastore.New(c)

	org := organization.New(db)
	org.Name = "eighty-nine-robotics"
	org.GetOrCreate("Name=", org.Name)

	u := user.New(db)
	u.Email = "jackie@eightyninerobotics.com "
	u.GetOrCreate("Email=", u.Email)
	u.FirstName = "Jackie"
	u.LastName = ""
	u.Organizations = []string{org.Id()}
	u.PasswordHash, _ = password.Hash("garage0)")
	u.MustPut()

	org.FullName = "EIGHTY NINE ROBOTICS"
	org.Owners = []string{u.Id()}
	org.Websites = []website.Website{website.Website{Type: website.Production, Url: "http://www.eightyninerobotics.com/"}}
	org.SecretKey = []byte("JuMVsRp26EzRXO9MzrXQXvH35XAW1W1E")
	org.AddDefaultTokens()

	org.Fees.Card.Flat = 50
	org.Fees.Card.Percent = 0.05
	org.Fees.Affiliate.Flat = 30
	org.Fees.Affiliate.Percent = 0.30

	// Save org into default namespace
	org.MustPut()

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

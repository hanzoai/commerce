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
	org.AddDefaultTokens()
	org.Fee = 0.05

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

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

var Myle = New("myle", func(c *gin.Context) *organization.Organization {
	db := datastore.New(c)

	org := organization.New(db)
	org.Name = "myle"
	org.GetOrCreate("Name=", org.Name)

	u := user.New(db)
	u.Email = "myle@verus.io"
	u.GetOrCreate("Email=", u.Email)
	u.FirstName = "Michael"
	u.LastName = "Walker"
	u.Organizations = []string{org.Id()}
	u.PasswordHash, _ = password.Hash("mylepassword!")
	u.Put()

	org.FullName = "Myle"
	org.Owners = []string{u.Id()}
	org.Website = "http://www.getmyle.com"
	org.SecretKey = []byte("197brMavJ20e3Q4rTFVpXu2IMESCu9vM")
	org.AddDefaultTokens()

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

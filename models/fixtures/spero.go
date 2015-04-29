package fixtures

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/auth/password"
	"crowdstart.io/datastore"
	"crowdstart.io/models/namespace"
	"crowdstart.io/models/organization"
	"crowdstart.io/models/user"

	"crowdstart.io/util/log"
)

var Spero = New("spero", func(c *gin.Context) *organization.Organization {
	db := datastore.New(c)

	org := organization.New(db)
	org.Name = "spero"
	org.GetOrCreate("Name=", org.Name)

	u := user.New(db)
	u.Email = "spero@verus.io"
	u.GetOrCreate("Email=", u.Email)
	u.FirstName = "Michael"
	u.LastName = "Walker"
	u.Organizations = []string{org.Id()}
	u.PasswordHash, _ = password.Hash("speropassword!")
	u.Put()

	org.FullName = "spero"
	org.Owners = []string{u.Id()}
	org.Website = "http://www.speroaudio.com"
	org.SecretKey = []byte("yW83JZGLjkGJE2gMfB4i0bwEoP03yJa5")
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

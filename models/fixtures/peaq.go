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

var PEAQAudio = New("peaqaudio", func(c *gin.Context) *organization.Organization {
	db := datastore.New(c)

	org := organization.New(db)
	org.Name = "peaqaudio"
	org.GetOrCreate("Name=", org.Name)

	u := user.New(db)
	u.Email = "cival@peaqaudio.com"
	u.GetOrCreate("Email=", u.Email)
	u.FirstName = "Cival"
	u.LastName = ""
	u.Organizations = []string{org.Id()}
	u.PasswordHash, _ = password.Hash("peaqpassword!")
	u.Put()

	org.FullName = "PEAQAudio"
	org.Owners = []string{u.Id()}
	org.Website = "http://peaqaudio.com"
	org.SecretKey = []byte("3kfmczo801fdmur0QtOCRZptNfRNV0uNexi")
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

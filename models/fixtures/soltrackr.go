package fixtures

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/auth/password"
	"hanzo.io/datastore"
	"hanzo.io/models/namespace"
	"hanzo.io/models/organization"
	"hanzo.io/models/user"
	"hanzo.io/util/log"
)

var _ = New("soltrackr", func(c *context.Context) *organization.Organization {
	db := datastore.New(c)

	org := organization.New(db)
	org.Name = "soltrackr"
	org.GetOrCreate("Name=", org.Name)

	u := user.New(db)
	u.Email = "david@soltrackr.com"
	u.GetOrCreate("Email=", u.Email)
	u.FirstName = "David"
	u.LastName = "Nam"
	u.Organizations = []string{org.Id()}
	u.PasswordHash, _ = password.Hash("soltrackrpassword!")
	u.MustPut()

	org.FullName = "SolTrackr Inc"
	org.Owners = []string{u.Id()}
	org.Website = "http://www.soltrackr.com/"
	org.SecretKey = []byte("KuMWsRq26FzRYO9NzsXRXwH35YAX2X5F")
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

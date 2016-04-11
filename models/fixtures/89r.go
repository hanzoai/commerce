package fixtures

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/auth/password"
	"crowdstart.com/datastore"
	"crowdstart.com/models/namespace"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/user"
)

var EightyNineRobotics = New("89r", func(c *gin.Context) *organization.Organization {
	db := datastore.New(c)

	org := organization.New(db)
	org.Name = "EIGHTY NINE ROBOTICS"
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
	org.Website = "http://www.eightyninerobotics.com/"
	org.SecretKey = []byte("JuMVsRp26EzRXO9MzrXQXvH35XAW1W1E")
	org.AddDefaultTokens()
	org.Fee = 0.05

	// Save org into default namespace
	org.MustPut()

	// Save namespace so we can decode keys for this organization later
	ns := namespace.New(db)
	ns.Name = org.Name
	ns.GetOrCreate("Name=", org.Name)

	ns.IntId = org.Key().IntID()
	ns.MustPut()

	return org
})

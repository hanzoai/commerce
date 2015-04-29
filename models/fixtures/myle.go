package fixtures

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/auth/password"
	"crowdstart.io/datastore"
	"crowdstart.io/models/organization"
	"crowdstart.io/models/user"
)

var Myle = New("myle", func(c *gin.Context) *organization.Organization {
	db := datastore.New(c)

	org := organization.New(db)
	org.Name = "myle"
	org.GetOrCreate("Name=", org.Name)

	u := user.New(db)
	u.Email = "dev@hanzo.ai"
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

	return org
})

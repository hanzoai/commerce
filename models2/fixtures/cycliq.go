package fixtures

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/auth2/password"
	"crowdstart.io/datastore"
	"crowdstart.io/models2/organization"
	"crowdstart.io/models2/user"
)

func Cycliq(c *gin.Context) *organization.Organization {
	db := datastore.New(c)

	org := organization.New(db)
	org.Name = "cycliq"
	org.GetOrCreate("Name=", org.Name)

	u := user.New(db)
	u.Email = "andrew@cycliq.com"
	u.GetOrCreate("Email=", u.Email)
	u.FirstName = "Andrew"
	u.LastName = "Hagen"
	u.Organizations = []string{org.Id()}
	u.PasswordHash, _ = password.Hash("cycliqpassword!")
	u.Put()

	u2 := user.New(db)
	u2.Email = "ac@theblackeyeproject.co.uk"
	u2.GetOrCreate("Email=", u2.Email)
	u2.FirstName = "Andy"
	u2.LastName = "Copely"
	u2.Organizations = []string{org.Id()}
	u2.PasswordHash, _ = password.Hash("cycliqpassword!")
	u2.Put()

	org.FullName = "Cycliq"
	org.Owners = []string{u.Id(), u2.Id()}
	org.Website = "http://cycliq.com"
	org.SecretKey = []byte("3kfmczo801fdmur0QtOCRZptNfRNV0uNexi")
	org.AddDefaultTokens()

	// Save org into default namespace
	org.Put()

	return org
}

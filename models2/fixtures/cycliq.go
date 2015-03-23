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

	user := user.New(db)
	user.Email = "ac@theblackeyeproject.co.uk"
	user.GetOrCreate("Email=", user.Email)
	user.FirstName = "Andy"
	user.LastName = "Copely"
	user.PasswordHash, _ = password.Hash("cycliqpass")

	org := organization.New(db)
	org.Name = "cycliq"
	org.GetOrCreate("Name=", org.Name)

	org.FullName = "Cycliq"
	org.OwnerId = user.Id()
	org.Website = "http://cycliq.com"
	org.SecretKey = []byte("3kfmczo801fdmur0QtOCRZptNfRNV0uNexi")
	org.AddDefaultTokens()

	// Save org into default namespace
	org.Put()

	// ..and also save org/user into org's namespace
	ctx := org.Namespace(c)
	user.SetContext(ctx)
	org.SetContext(ctx)
	org.Put()
	user.Put()

	return org
}

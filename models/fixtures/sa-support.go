package fixtures

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/auth/password"
	"crowdstart.com/datastore"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/user"
)

var StonedSupport = New("stoned-support", func(c *gin.Context) *organization.Organization {
	db := datastore.New(c)

	org := organization.New(db)
	org.Name = "stoned"
	org.GetOrCreate("Name=", org.Name)

	u := user.New(db)
	u.Email = "gina@verus.io"
	u.GetOrCreate("Email=", u.Email)
	u.FirstName = "Gina"
	u.LastName = "Kelling"
	u.Organizations = []string{org.Id()}
	u.PasswordHash, _ = password.Hash("veruspassword!")
	u.Put()

	u2 := user.New(db)
	u2.Email = "dev@hanzo.ai"
	u2.GetOrCreate("Email=", u2.Email)
	u2.FirstName = "Ali"
	u2.LastName = "Kelling"
	u2.Organizations = []string{org.Id()}
	u2.PasswordHash, _ = password.Hash("veruspassword!")
	u2.Put()

	return org
})

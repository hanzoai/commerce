package fixtures

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/auth/password"
	"crowdstart.com/datastore"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/user"
)

var _ = New("stoned-shipstation", func(c *gin.Context) *organization.Organization {
	db := datastore.New(c)

	org := organization.New(db)
	org.Name = "stoned"
	org.GetOrCreate("Name=", org.Name)

	u := user.New(db)
	u.Email = "shipstation@stoned.audio"
	u.GetOrCreate("Email=", u.Email)
	u.FirstName = "Shipstation"
	u.LastName = ""
	u.Organizations = []string{org.Id()}
	u.PasswordHash, _ = password.Hash("ZGvb49Pik8Ms!")
	u.Put()

	org.Admins = append(org.Admins, u.Id())
	org.Put()

	return org
})

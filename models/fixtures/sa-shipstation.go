package fixtures

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/auth/password"
	"hanzo.io/datastore"
	"hanzo.io/models/organization"
	"hanzo.io/models/user"
)

var _ = New("stoned-shipstation", func(c *context.Context) *organization.Organization {
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

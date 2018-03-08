package fixtures

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/auth/password"
	"hanzo.io/datastore"
	"hanzo.io/models/organization"
	"hanzo.io/models/user"
)

var StonedSupport = New("stoned-support", func(c *gin.Context) *organization.Organization {
	db := datastore.New(c)

	org := organization.New(db)
	org.Name = "stoned"
	org.GetOrCreate("Name=", org.Name)

	datastore.RunInTransaction(db.Context, func(db *datastore.Datastore) error {
		u := user.New(db)
		u.Email = "gina@verus.io"
		u.GetOrCreate("Email=", u.Email)
		u.FirstName = "Gina"
		u.LastName = "Kelling"
		u.Organizations = []string{org.Id()}
		u.PasswordHash, _ = password.Hash("veruspassword!")
		u.MustPut()
		return nil
	}, &datastore.TransactionOptions{XG: true})

	datastore.RunInTransaction(db.Context, func(db *datastore.Datastore) error {
		u2 := user.New(db)
		u2.Email = "dev@hanzo.ai"
		u2.GetOrCreate("Email=", u2.Email)
		u2.FirstName = "Ali"
		u2.LastName = "Kelling"
		u2.Organizations = []string{org.Id()}
		u2.PasswordHash, _ = password.Hash("veruspassword!")
		u2.MustPut()
		return nil
	}, &datastore.TransactionOptions{XG: true})

	return org
})

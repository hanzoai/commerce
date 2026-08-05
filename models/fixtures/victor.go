package fixtures

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/auth/password"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/user"
)

var _ = New("victor", func(c *zip.Ctx) *organization.Organization {
	db := datastore.New(c.Context())

	// Our fake T-shirt company
	org := organization.New(db)
	org.Name = "suchtees"
	org.GetOrCreate("Name=", org.Name)

	// Such tees owner & operator
	u := user.New(db)
	u.Email = "victor@suchtees.com"
	u.GetOrCreate("Email=", u.Email)

	u.FirstName = "Victor"
	u.LastName = "Canera"
	u.PasswordHash, _ = password.Hash("78v6gvKhrkEwWZsJ")
	u.Organizations = []string{org.Id()}
	u.MustPut()

	org.Owners = append(org.Owners, u.Id())
	org.MustPut()

	return org
})

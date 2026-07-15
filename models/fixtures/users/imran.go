package fixtures

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/auth/password"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/user"

	. "github.com/hanzoai/commerce/models/fixtures"
)

var ImranForLuckyBets = New("imran-for-luckybets", func(c *zip.Ctx) {
	db := datastore.New(c.Context())

	org := organization.New(db)
	org.Name = "luckybets"
	org.GetOrCreate("Name=", org.Name)

	u := user.New(db)
	u.Email = "dev@hanzo.ai"
	u.GetOrCreate("Email=", u.Email)
	u.FirstName = "Imran"
	u.LastName = "Hameed"
	u.Organizations = []string{org.Id()}
	u.PasswordHash, _ = password.Hash("NvK27bzuKmqBeBBH")
	u.MustUpdate()

	org.Owners = append(org.Owners, u.Id())
	org.MustUpdate()
})

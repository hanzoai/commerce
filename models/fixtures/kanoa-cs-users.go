package fixtures

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/auth/password"
	"hanzo.io/datastore"
	"hanzo.io/models/organization"
	"hanzo.io/models/user"
)

var KanoaCSUsers = New("kanoa-cs-users", func(c *context.Context) {
	db := datastore.New(c)

	org := organization.New(db)
	org.Name = "kanoa"
	org.GetOrCreate("Name=", org.Name)

	// u := user.New(db)
	// u.Email = "dev@hanzo.ai"
	// u.GetOrCreate("Email=", u.Email)
	// u.FirstName = "Lorenzo"
	// u.LastName = "Castillo"
	// u.Organizations = []string{org.Id()}
	// u.PasswordHash, _ = password.Hash("1Kanoa23")
	// u.Put()

	// u := user.New(db)
	// u.Email = "jordan@getkanoa.com"
	// u.GetOrCreate("Email=", u.Email)
	// u.FirstName = "Jordan"
	// u.LastName = "Shou"
	// u.Organizations = []string{org.Id()}
	// u.PasswordHash, _ = password.Hash("1Kanoa23")
	// u.Put()

	u := user.New(db)
	u.Email = "kyle@getkanoa.com"
	u.GetOrCreate("Email=", u.Email)
	u.FirstName = "Kyle"
	u.LastName = "Morrison"
	u.Organizations = []string{org.Id()}
	u.PasswordHash, _ = password.Hash("1Kanoa23")
	u.Put()
})

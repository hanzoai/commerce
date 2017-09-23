package fixtures

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/auth/password"
	"hanzo.io/datastore"
	"hanzo.io/models/organization"
	"hanzo.io/models/user"

	. "hanzo.io/models/fixtures"
)

var KirbyForStonedAndSuchTees = New("kirby-for-stoned-and-suchtees", func(c *gin.Context) {
	db := datastore.New(c)

	org := organization.New(db)
	org.Name = "stoned"
	org.GetOrCreate("Name=", org.Name)

	org2 := organization.New(db)
	org2.Name = "suchtees"
	org2.GetOrCreate("Name=", org.Name)

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
	u.Email = "dev@hanzo.ai"
	u.GetOrCreate("Email=", u.Email)
	u.FirstName = "Kirby"
	u.LastName = "Chamblin"
	u.Organizations = []string{org.Id(), org2.Id()}
	u.PasswordHash, _ = password.Hash("H4tguADoBH")
	u.Put()
})

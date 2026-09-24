package fixtures

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/auth/password"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/user"
)

var User = New("user", func(c *zip.Ctx) *user.User {
	db := datastore.New(c.Context())

	// Such tees owner & operator
	usr := user.New(db)
	usr.Email = "dev@hanzo.ai"
	usr.GetOrCreate("Email=", usr.Email)

	usr.FirstName = "Jackson"
	usr.LastName = "Shirts"
	usr.Phone = "(999) 999-9999"
	usr.PasswordHash, _ = password.Hash("suchtees")
	usr.MustPut()
	return usr
})

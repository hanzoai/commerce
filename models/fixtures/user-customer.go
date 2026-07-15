package fixtures

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/auth/password"
	"github.com/hanzoai/commerce/models/user"
)

var UserCustomer = New("user-customer", func(c *zip.Ctx) *user.User {
	db := getNamespaceDb(c)

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

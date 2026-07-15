package middleware

import (
	"errors"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/user"
	"github.com/hanzoai/commerce/util/json/http"
)

func AccountRequired() zip.Handler {
	return func(c *zip.Ctx) error {
		tok := GetToken(c)

		id := tok.UserId
		if id == "" {
			return http.Fail(c, 403, "Access Denied", errors.New("Access Denied"))
		}

		org := GetOrganization(c)
		db := datastore.New(org.Namespaced(c.Context()))
		u := user.New(db)

		if err := u.GetById(id); err != nil {
			return http.Fail(c, 403, "Access Denied", errors.New("Access Denied"))
		}

		c.Locals("user", u)
		return c.Next()
	}
}

func GetUser(c *zip.Ctx) *user.User {
	return c.Locals("user").(*user.User)
}

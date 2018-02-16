package middleware

import (
	"errors"

	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/models/user"
	"hanzo.io/util/json/http"
)

func AccountRequired() gin.HandlerFunc {
	return func(c *context.Context) {
		tok := GetToken(c)

		id, ok := tok.Get("user-id").(string)
		if !ok {
			http.Fail(c, 403, "Access Denied", errors.New("Access Denied"))
			return
		}

		org := GetOrganization(c)
		db := datastore.New(org.Namespaced(c))
		u := user.New(db)

		if err := u.GetById(id); err != nil {
			http.Fail(c, 403, "Access Denied", errors.New("Access Denied"))
			return
		}

		c.Set("user", u)
	}
}

func GetUser(c *context.Context) *user.User {
	return c.MustGet("user").(*user.User)
}

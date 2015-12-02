package middleware

import (
	"errors"

	"github.com/gin-gonic/gin"

	"crowdstart.com/datastore"
	"crowdstart.com/models/user"
	"crowdstart.com/util/json/http"
)

func AccountRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		tok := GetToken(c)

		id, ok := tok.Get("user-id").(string)
		if !ok {
			http.Fail(c, 403, "Access Denied", errors.New("Access Denied"))
			return
		}

		org := GetOrganization(c)
		db := datastore.New(org.Namespace(c))
		u := user.New(db)

		if err := u.GetById(id); err != nil {
			http.Fail(c, 403, "Access Denied", errors.New("Access Denied"))
			return
		}

		c.Set("user", u)
	}
}

func GetUser(c *gin.Context) *user.User {
	return c.MustGet("user").(*user.User)
}

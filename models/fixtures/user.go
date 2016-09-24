package fixtures

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/auth/password"
	"crowdstart.com/datastore"
	"crowdstart.com/models/user"
)

var User = New("user", func(c *gin.Context) *user.User {
	db := datastore.New(c)

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

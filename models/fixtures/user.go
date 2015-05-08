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
	user := user.New(db)
	user.Email = "dev@hanzo.ai"
	user.GetOrCreate("Email=", user.Email)

	user.FirstName = "Jackson"
	user.LastName = "Shirts"
	user.Phone = "(999) 999-9999"
	user.PasswordHash, _ = password.Hash("suchtees")
	user.MustPut()
	return user
})

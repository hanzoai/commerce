package fixtures

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/auth"
	"crowdstart.io/datastore"
	"crowdstart.io/models2/user"
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
	user.PasswordHash = auth.HashPassword("suchtees")
	user.MustPut()
	return user
})

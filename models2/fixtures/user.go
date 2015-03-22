package fixtures

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/auth"
	"crowdstart.io/datastore"
	"crowdstart.io/models2/user"
)

func User(c *gin.Context) *user.User {
	db := datastore.New(c)

	// Owner for this organization
	user := user.New(db)
	user.Email = "dev@hanzo.ai"
	user.GetOrCreate("Email=", user.Email)

	user.FirstName = "Jackson"
	user.LastName = "Shirts"
	user.Phone = "(999) 999-9999"
	user.PasswordHash = auth.HashPassword("suchtees")
	user.MustPut()
	return user
}

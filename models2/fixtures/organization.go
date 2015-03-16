package fixtures

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/auth"
	"crowdstart.io/datastore"
	"crowdstart.io/models2/organization"
	"crowdstart.io/models2/user"
	"crowdstart.io/util/task"
)

var _ = task.Func("models2-fixtures-organization", func(c *gin.Context) {
	db := datastore.New(c)

	// Owner for this organization
	user := user.New(db)

	user.FirstName = "Jackson"
	user.LastName = "Shirts"
	user.Phone = "(999) 999-9999"
	user.Email = "dev@hanzo.ai"
	user.PasswordHash = auth.HashPassword("suchtees")
	user.GetOrCreate("Email=", user.Email)

	// Our fake T-shirt company
	org := organization.New(db)

	org.Name = "suchtees"
	org.FullName = "Such Tees, Inc."
	org.Owners = []string{user.Id()}
	org.Website = "http://suchtees.com"
	org.GetOrCreate("Name=", org.Name)
})

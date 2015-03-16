package fixtures

import (
	"github.com/gin-gonic/gin"

	"appengine"

	"crowdstart.io/auth"
	"crowdstart.io/datastore"
	"crowdstart.io/middleware"
	"crowdstart.io/models2/organization"
	"crowdstart.io/models2/user"
	"crowdstart.io/util/task"
)

var _ = task.Func("models2-fixtures-organization", func(c *gin.Context) {
	db := datastore.New(c)

	// Owner for this organization
	user := user.New(db)
	user.Email = "dev@hanzo.ai"
	user.GetOrCreate("Email=", user.Email)

	user.FirstName = "Jackson"
	user.LastName = "Shirts"
	user.Phone = "(999) 999-9999"
	user.PasswordHash = auth.HashPassword("suchtees")
	user.Put()

	// Our fake T-shirt company
	org := organization.New(db)
	org.Name = "suchtees"
	org.GetOrCreate("Name=", org.Name)

	org.FullName = "Such Tees, Inc."
	org.Owners = []string{user.Id()}
	org.Website = "http://suchtees.com"
	org.Put()

	// Save into org's namespace
	ctx := middleware.GetAppEngine(c)
	ctx, _ = appengine.Namespace(ctx, org.Id())
	org.SetContext(ctx)
	org.Put()

	user.SetContext(ctx)
	user.Put()
})

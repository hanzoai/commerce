package account

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/middleware"
	"hanzo.io/models/user"
	"hanzo.io/util/json/http"
)

func exists(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))
	email := c.Params.ByName("email")

	usr := user.New(db)

	if err := usr.GetByEmail(email); err == nil {
		http.Render(c, 200, gin.H{"exists": true})
	} else {
		http.Render(c, 200, gin.H{"exists": false})
	}
}

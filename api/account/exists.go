package account

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/middleware"
	"hanzo.io/models/user"
	"hanzo.io/util/json/http"
)

func exists(c *context.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))
	emailorusername := c.Params.ByName("emailorusername")

	usr := user.New(db)

	if err := usr.GetByEmail(emailorusername); err == nil {
		http.Render(c, 200, gin.H{"exists": true})
	} else if err := usr.GetByUsername(emailorusername); err == nil {
		http.Render(c, 200, gin.H{"exists": true})
	} else {
		http.Render(c, 200, gin.H{"exists": false})
	}
}

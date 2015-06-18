package account

import (
	"errors"

	"github.com/gin-gonic/gin"

	"crowdstart.com/datastore"
	"crowdstart.com/middleware"
	"crowdstart.com/models/user"
	"crowdstart.com/util/json/http"
)

func exists(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespace(c))
	email := c.Params.ByName("email")

	usr := user.New(db)

	if err := usr.GetByEmail(email); err == nil {
		http.Fail(c, 400, "Email is in use", errors.New("Email is in use"))
		return
	}

	http.Render(c, 200, gin.H{"status": "ok"})
}

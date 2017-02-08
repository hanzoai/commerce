package form

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/models/form"
	"hanzo.io/models/organization"
	formtype "hanzo.io/models/types/form"
	"hanzo.io/util/json/http"
)

// handle form submissions
func handleForm(c *gin.Context) {
	id := c.Params.ByName("formid")
	db := datastore.New(c)

	f := form.New(db)

	// Set key and namespace correctly
	f.SetKey(id)
	f.SetNamespace(f.Key().Namespace())
	db.Context = f.Db.Context

	// Get organization for form
	org := organization.New(db)
	org.GetById(f.Key().Namespace())

	// Mailing list doesn't exist
	if err := f.Get(); err != nil {
		http.Fail(c, 404, fmt.Sprintf("Failed to retrieve mailing list '%v': %v", id, err), err)
		return
	}

	switch f.Type {
	case formtype.Subscribe:
		subscribe(c, db, org, f)
	case formtype.Submit:
		submit(c, db, org, f)
	}
}

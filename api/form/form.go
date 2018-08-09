package form

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/models/form"
	"hanzo.io/models/organization"
	"hanzo.io/models/types/form"
	"hanzo.io/util/json/http"
)

// handle form submissions
func handleForm(c *gin.Context) {
	db := datastore.New(c)
	id := c.Params.ByName("formid")
	org := organization.New(db)
	f := form.New(db)

	// Set mailinglist key
	f.SetKey(id)

	// Reset namespace to organization's
	f.SetNamespace(f.Key().Namespace())

	// Get namespaced db
	db = f.Datastore()

	// Get organization for mailinglist
	org.GetById(f.Key().Namespace())

	// Mailing list doesn't exist
	if err := f.Get(nil); err != nil {
		http.Fail(c, 404, fmt.Sprintf("Failed to retrieve form '%v': %v", id, err), err)
		return
	}

	switch f.Type {
	case form.Submit:
		submit(c, db, org, ml)
	default:
		//case form.Subscribe:
		subscribe(c, db, org, ml)
	}
}

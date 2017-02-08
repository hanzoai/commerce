package form

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/models/mailinglist"
	"hanzo.io/models/organization"
	"hanzo.io/models/types/form"
	"hanzo.io/util/json/http"
)

// handle form submissions
func handleForm(c *gin.Context) {
	db := datastore.New(c)
	id := c.Params.ByName("mailinglistid")
	org := organization.New(db)
	ml := mailinglist.New(db)

	// Set mailinglist key
	ml.SetKey(id)

	// Reset namespace to organization's
	ml.SetNamespace(ml.Key().Namespace())

	// Get namespaced db
	db = ml.Datastore()

	// Get organization for mailinglist
	org.GetById(ml.Key().Namespace())

	// Mailing list doesn't exist
	if err := ml.Get(nil); err != nil {
		http.Fail(c, 404, fmt.Sprintf("Failed to retrieve mailing list '%v': %v", id, err), err)
		return
	}

	switch ml.Type {
	case form.Subscribe:
		subscribe(c, db, org, ml)
	case form.Submit:
		submit(c, db, org, ml)
	}
}

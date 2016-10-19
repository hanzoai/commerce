package form

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"crowdstart.com/datastore"
	"crowdstart.com/models/mailinglist"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/types/form"
	"crowdstart.com/util/json/http"
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

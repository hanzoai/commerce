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
	id := c.Params.ByName("mailinglistid")
	db := datastore.New(c)

	ml := mailinglist.New(db)

	// Set key and namespace correctly
	ml.SetKey(id)
	ml.SetNamespace(ml.Key().Namespace())
	db.Context = ml.Db.Context

	// Get organization for mailinglist
	org := organization.New(db)
	org.GetById(ml.Key().Namespace())

	// Mailing list doesn't exist
	if err := ml.Get(); err != nil {
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

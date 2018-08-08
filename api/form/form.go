package form

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/models/mailinglist"
	"hanzo.io/models/organization"
	"hanzo.io/models/types/form"
	"hanzo.io/util/json/http"
	"hanzo.io/log"
)

// handle form submissions
func handleForm(c *gin.Context) {
	db := datastore.New(c)
	id := c.Params.ByName("mailinglistid")
	org := organization.New(db)
	ml := mailinglist.New(db)

	// Set mailinglist key
	ml.SetKey(id)

	// Get namepsace
	ns := ml.Key().Namespace()

	// Get organization for mailinglist
	if err := org.GetById(ns); err != nil {
		log.Error("Organization not found: %v ?= %v,  %v", ns, org.Name, org.Id_, c)
		http.Fail(c, 404, fmt.Sprintf("Failed to retrieve organization '%v': %v", ns, err), err)
		return
	}
	log.Info("Organization: %v ?= %v,  %v", ns, org.Name, org.Id_, c)

	// Reset namespace to organization's
	ml.SetNamespace(ns)

	// Mailing list doesn't exist
	if err := ml.Get(nil); err != nil {
		http.Fail(c, 404, fmt.Sprintf("Failed to retrieve mailing list '%v': %v", id, err), err)
		return
	}

	// Get namespaced db
	db = ml.Datastore()

	switch ml.Type {
	case form.Submit:
		submit(c, db, org, ml)
	default:
		//case form.Subscribe:
		subscribe(c, db, org, ml)
	}
}

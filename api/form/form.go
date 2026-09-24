package form

import (
	"fmt"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/form"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/util/json/http"
)

// handle form submissions
func handleForm(c *zip.Ctx) error {
	db := datastore.New(c.Context())
	id := c.Param("formid")
	org := organization.New(db)
	f := form.New(db)

	// Set form key
	f.SetKey(id)

	// Get namepsace
	ns := f.Key().Namespace()

	// Get organization for form
	if err := org.GetById(ns); err != nil {
		log.Error("Organization not found: %v ?= %v,  %v", ns, org.Name, org.Id_, c)
		return http.Fail(c, 404, fmt.Sprintf("Failed to retrieve organization '%v': %v", ns, err), err)
	}
	log.Info("Organization: %v ?= %v,  %v", ns, org.Name, org.Id_, c)

	// Set namespace to match organization's
	f.SetNamespace(ns)

	// Mailing list doesn't exist
	if err := f.Get(nil); err != nil {
		return http.Fail(c, 404, fmt.Sprintf("Failed to retrieve form '%v': %v", id, err), err)
	}

	// Get namespaced db
	db = f.Datastore()

	switch f.Type {
	case form.Submit:
		return submit(c, db, org, f)
	default:
		//case form.Subscribe:
		return subscribe(c, db, org, f)
	}
}

package form

import (
	"fmt"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/form"
)

func Js(c *zip.Ctx) error {
	id := c.Param("formid")
	db := datastore.New(c.Context())

	f := form.New(db)

	// Set key and namespace correctly
	f.SetKey(id)
	log.Debug("form: %v", f, c)
	log.Debug("key: %v", f.Key(), c)
	namespace := f.Key().Namespace()
	log.Warn("namespace: %v", namespace, c)
	f.SetNamespace(namespace)

	if err := f.Get(nil); err != nil {
		log.Error("Failed to retrieve form '%s' in namespace '%s': %v", id, namespace, err, c)
		return c.String(404, fmt.Sprintf("Failed to retrieve form '%v': %v", id, err))
	}

	c.SetHeader("Content-Type", "application/javascript")
	return c.String(200, f.Js())
}

package migrations

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/models/namespace"
	"hanzo.io/util/log"
)

var _ = New("add-missing-namespace", func(c *context.Context) []interface{} {
	db := datastore.New(c)
	ns := namespace.New(db)
	ns.Name = "4050001"
	ns.IntId = 4050001
	err := ns.Put()
	if err != nil {
		log.Warn("Failed to put namespace: %v", err)
	}
	return NoArgs
})

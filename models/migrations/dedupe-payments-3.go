package migrations

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/models/payment"
	"hanzo.io/util/log"
)

var _ = New("dedupe-payments-3", func(c *gin.Context) []interface{} {
	db := datastore.New(c)
	db.SetNamespace("bellabeat")

	keys, err := payment.Query(db).Filter("Deleted=", true).Limit(500).GetKeys()
	if err != nil {
		log.Panic("Failed to get keys for deleted payments: %v", err, c)
	}

	log.Debug("Deleting %s keys", len(keys), c)

	if err := db.DeleteMulti(keys); err != nil {
		log.Warn("Failed to delete keys: %v", err, c)
	}

	return NoArgs
})

package migrations

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/payment"
)

var _ = New("dedupe-payments-3", func(c *zip.Ctx) []interface{} {
	db := datastore.New(c.Context())
	db.SetNamespace("kanoa")

	keys, err := payment.Query(db).Filter("Deleted=", true).KeysOnly().Limit(500).GetAll(nil)
	if err != nil {
		log.Panic("Failed to get keys for deleted payments: %v", err, c)
	}

	for len(keys) > 0 {
		log.Debug("Deleting %s keys", len(keys), c)

		if err := db.DeleteMulti(keys); err != nil {
			log.Warn("Failed to delete keys: %v", err, c)
		}

		keys, err = payment.Query(db).Filter("Deleted=", true).KeysOnly().Limit(500).GetAll(nil)
		if err != nil {
			log.Panic("Failed to get keys for deleted payments: %v", err, c)
		}
	}

	return NoArgs
})

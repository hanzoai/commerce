package migrations

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/datastore"
	"crowdstart.com/models/payment"
	"crowdstart.com/util/log"
)

var _ = New("dedupe-payments-3", func(c *gin.Context) []interface{} {
	db := datastore.New(c)
	db.SetNamespace("bellabeat")

	keys, err := payment.Query(db).Filter("Deleted=", true).KeysOnly().Limit(10000).GetAll(nil)
	if err != nil {
		log.Panic("Failed to get keys for deleted payments: %v", err, c)
	}

	if err := db.DeleteMulti(keys); err != nil {
		log.Debug("Failed to delete keys: %v", err, c)
	}

	return NoArgs
})

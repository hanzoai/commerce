package migrations

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/models/user"
	"hanzo.io/log"
)

var _ = New("clean-up-dupe-users", func(c *gin.Context) []interface{} {
	db := datastore.New(c)
	db.SetNamespace("kanoa")

	keys, err := user.Query(db).Filter("Deleted=", true).KeysOnly().Limit(500).GetAll(nil)
	if err != nil {
		log.Panic("Failed to get keys for deleted users: %v", err, c)
	}

	for len(keys) > 0 {
		log.Debug("Deleting %s keys", len(keys), c)

		if err := db.DeleteMulti(keys); err != nil {
			log.Warn("Failed to delete keys: %v", err, c)
		}

		keys, err = user.Query(db).Filter("Deleted=", true).KeysOnly().Limit(500).GetAll(nil)
		if err != nil {
			log.Panic("Failed to get keys for deleted users: %v", err, c)
		}
	}

	return NoArgs
})

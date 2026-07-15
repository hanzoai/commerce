package migrations

import (
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/token"

	ds "github.com/hanzoai/commerce/datastore"
)

var _ = New("update-unused-tokens",
	func(c *zip.Ctx) []interface{} {
		c.Locals("namespace", "bellabeat")

		return NoArgs
	},
	func(db *ds.Datastore, tok *token.Token) {
		if !tok.Used && tok.Expired() {
			now := time.Now()
			tok.Used = false
			tok.Expires = now.Add(30 * 24 * time.Hour)
			if err := tok.Put(); err != nil {
				log.Error(err, db.Context)
			}
		}
	},
)

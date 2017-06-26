package migrations

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/models/organization"
	"hanzo.io/models/store"
	"hanzo.io/thirdparty/mailchimp"
	"hanzo.io/util/log"

	ds "hanzo.io/datastore"
)

var _ = New("cover-mailchimp-store",
	func(c *gin.Context) []interface{} {
		c.Set("namespace", "cover")

		db := ds.New(c)
		org := organization.New(db)
		if _, err := org.Query().Filter("Name=", "cover").Get(); err != nil {
			panic(err)
		}
		return []interface{}{org.Mailchimp.APIKey, org.DefaultStore}
	},
	func(db *ds.Datastore, stor *store.Store, apiKey, defaultStore string) {
		if apiKey == "" {
			log.Warn("No MailChimp API Key", db.Context)
			return
		}

		client := mailchimp.New(db.Context, apiKey)

		// Create new store
		if stor.Id() == defaultStore {
			stor.Mailchimp.ListId = "95dee09328"
		} else {
			stor.Mailchimp.ListId = "1fa3cc7c33"
		}
		stor.MustUpdate()

		if err := client.CreateStore(stor); err != nil {
			log.Error("Failed to create Mailchimp store: %v", err, db.Context)
		}
	},
)

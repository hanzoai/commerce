package tasks

import (
	"encoding/gob"

	"hanzo.io/datastore"
	"hanzo.io/models/analyticsidentifier"
	"hanzo.io/util/delay"
	"hanzo.io/util/log"

	"appengine"
)

func init() {
	gob.Register(&analyticsidentifier.AnalyticsIdentifier{})
}

var CohereIds = delay.Func("cohere-ids", func(ctx appengine.Context, id *analyticsidentifier.AnalyticsIdentifier) {
	db := datastore.New(ctx)

	id.Db = db
	id.Entity = id

	found := false

	err := db.RunInTransaction(func(db *datastore.Datastore) error {
		ids := make([]*analyticsidentifier.AnalyticsIdentifier, 0)
		if _, err := analyticsidentifier.Query(db).Filter("UUId=", id.UUId).GetAll(&ids); err != nil {
			log.Error("Failed trying to find AnalyticsIdentifier with UUId %v", id.UUId, ctx)
			return err
		}

		found = len(ids) > 0

		if id.UserId != "" {
			for _, id2 := range ids {
				id2.Db = db
				id2.Entity = id2

				id2.UserId = id.UserId
				id2.Update()
			}
		}

		return nil
	})

	if err == nil {
		return
	}
})

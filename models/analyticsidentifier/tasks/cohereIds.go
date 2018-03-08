package tasks

import (
	"context"
	"encoding/gob"

	"hanzo.io/datastore"
	"hanzo.io/log"
	"hanzo.io/models/analyticsidentifier"
	"hanzo.io/delay"
)

func init() {
	gob.Register(&analyticsidentifier.AnalyticsIdentifier{})
}

var CohereIds = delay.Func("cohere-ids", func(ctx context.Context, id *analyticsidentifier.AnalyticsIdentifier) {
	db := datastore.New(ctx)

	id.Db = db
	id.Entity = id

	found := false

	db.RunInTransaction(func(db *datastore.Datastore) error {
		ids := make([]*analyticsidentifier.AnalyticsIdentifier, 0)
		if _, err := analyticsidentifier.Query(db).Filter("UUId=", id.UUId).GetAll(&ids); err != nil {
			log.Error("Failed trying to find AnalyticsIdentifier with UUId %v", id.UUId, ctx)
			return err
		}

		found = len(ids) > 0

		userIdUpdates := id.UserId != ""

		newGAId := true
		newFBId := true

		// New GA Id check
		for _, id2 := range ids {
			if id.GAId == id2.GAId {
				newGAId = false
				break
			}
		}

		// New FacebookId check
		for _, id2 := range ids {
			if id.FBId == id2.FBId {
				newFBId = false
				break
			}
		}

		// New UserId check
		if userIdUpdates {
			for _, id2 := range ids {
				if id.UserId == id2.UserId {
					continue
				}

				id2.Db = db
				id2.Entity = id2

				id2.UserId = id.UserId
				id2.Update()
			}
		}

		if newGAId || newFBId {
			id.Create()
		}

		return nil
	}, nil)
})

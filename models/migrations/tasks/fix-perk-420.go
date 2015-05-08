package tasks

import (
	"strconv"
	"time"

	"crowdstart.io/datastore"
	"crowdstart.io/datastore/parallel"
	"crowdstart.io/models"
	"crowdstart.io/util/log"
)

var FixPerk420 = parallel.Task("fix-perk-420", func(db *datastore.Datastore, key datastore.Key, order models.Order) {
	LastMonth := time.Now().Add(-30 * 24 * time.Hour)
	if order.CreatedAt.Before(LastMonth) {
		return
	}

	cKey := db.NewKey("contribution", strconv.Itoa(int(key.IntID())), 0, nil)
	contribution := models.Contribution{}
	err := db.Get(cKey, &contribution)
	if err != nil {
		log.Error("No contribution with id %v", key.IntID(), db.Context)
		return
	}

	if contribution.Perk.Id != "AR1-2015" && contribution.Perk.Id != "" {
		return
	}

	contribution.Perk = models.Perks["AR1-2015"]
	db.Put(cKey, &contribution)
	if err != nil {
		log.Error("Could not put contribution with id %v", key.IntID(), db.Context)
	}
})

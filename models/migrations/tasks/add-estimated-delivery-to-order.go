package tasks

import (
	"strconv"
	"time"

	"crowdstart.io/datastore"
	"crowdstart.io/datastore/parallel"
	"crowdstart.io/models"
	"crowdstart.io/util/log"

	aeds "appengine/datastore"
)

var AddEstimateDeliveryToOrder = parallel.Task("add-estimated-delivery-to-orders", func(db *datastore.Datastore, key datastore.Key, order models.Order) {
	contributions := make([]models.Contribution, 0)
	var keys []*aeds.Key
	var err error

	// Get Contributions
	if keys, err = db.Query("contribution").Filter("UserId =", order.UserId).GetAll(db.Context, &contributions); err != nil {
		log.Error("Task has encountered error: %v", err, db.Context)
		return
	}

	// Set a default delivery date
	order.EstimatedDelivery = "HELMET: May 2015,DOGTAG: December 2014,XMAS CARD: Downloadable"

	// Loop over contributions, make sure the key Ids are the same before setting the date
	log.Debug("Contributions found %v", len(contributions))
	for i, contribution := range contributions {
		if id, err := strconv.Atoi(keys[i].StringID()); err == nil && int64(id) == key.IntID() {
			log.Debug("Contribution Type '%v'", contribution.Perk.Id, db.Context)

			order.EstimatedDelivery = contribution.Perk.EstimatedDelivery
			break
		}
	}

	order.UpdatedAt = time.Now()
	db.PutKind("order", key, &order)
})

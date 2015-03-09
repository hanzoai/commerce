package tasks

import (
	"strconv"
	"strings"
	"time"

	"crowdstart.io/datastore"
	"crowdstart.io/datastore/parallel"
	"crowdstart.io/models"
	"crowdstart.io/util/log"

	aeds "appengine/datastore"
)

var FixOrderPrice = parallel.Task("fix-order-price", func(db *datastore.Datastore, key datastore.Key, contribution models.Contribution) {
	// Ignore winter promo stuff
	if contribution.Perk.Id == "WINTER2014PROMO" {
		return
	}

	orders := make([]models.Order, 0)
	var keys []*aeds.Key
	var err error

	if keys, err = db.Query("order").Filter("UserId =", contribution.UserId).GetAll(db.Context, &orders); err != nil {
		log.Error("Task has encountered error: %v", err, db.Context)
		return
	}

	// Get Price from contribution
	price := contribution.Perk.Price
	tokens := strings.Split(price, " ")

	price = strings.TrimSpace(tokens[0])
	price = strings.Replace(price, "$", "", -1)
	price = strings.Replace(price, ",", "", -1)

	// Convert dollar price to centicents
	centicents, err := strconv.ParseInt(price, 10, 64)
	if err != nil {
		log.Error("Task has encountered error: %v", err, db.Context)
		return
	}

	centicents *= 10000

	for i, order := range orders {
		order.Subtotal = centicents
		order.UpdatedAt = time.Now()
		db.PutKind("order", keys[i], order)
	}
})

package tasks

import (
	"time"

	"crowdstart.io/datastore"
	"crowdstart.io/datastore/parallel"
	"crowdstart.io/models"
	"crowdstart.io/util/log"

	aeds "appengine/datastore"
)

var AddMissingOrder = parallel.Task("add-missing-orders-from-contribution", func(db *datastore.Datastore, key datastore.Key, contribution models.Contribution) {
	var orders []models.Order
	var keys []*aeds.Key
	var err error

	if keys, err = db.Query("orders").Filter("UserId=", contribution.UserId).GetAll(db.Context, &orders); err != nil {
		log.Error("Task has encountered error: %v", err, db.Context)
		return
	}

	foundIndex := -1
	//Check to see if there is a matching order
	for i, order := range orders {
		if contribution.Perk.Title == "AR-1 HOLIDAY PREORDER" &&
			order.Items[0].Slug() == "ar-1" &&
			order.Items[1].Slug() == "card-winter2014promo" &&
			order.Items[2].Slug() == "dogtag-winter2014promo" &&
			order.Items[0].Quantity == contribution.Perk.HelmetQuantity &&
			order.Items[1].Quantity == contribution.Perk.HelmetQuantity &&
			order.Items[2].Quantity == contribution.Perk.HelmetQuantity {
			foundIndex = i
		} else if order.Items[0].Slug() == "ar-1" &&
			order.Items[1].Slug() == "t-shirt" &&
			order.Items[2].Slug() == "hat" &&
			order.Items[0].Quantity == contribution.Perk.HelmetQuantity &&
			order.Items[1].Quantity == contribution.Perk.GearQuantity &&
			order.Items[2].Quantity == contribution.Perk.GearQuantity {
			foundIndex = i
		}
	}

	// Upsert if found, insert new order if not found
	if foundIndex != -1 {
		// Update the email for book keeping
		orders[foundIndex].Email = contribution.Email
		db.PutKey("order", keys[foundIndex], orders[foundIndex])
	} else {
		user := new(models.User)
		db.Get(contribution.UserId, user)

		order := &models.Order{}
		order.UserId = user.Id
		order.Email = user.Email
		order.ShippingAddress = user.ShippingAddress
		order.BillingAddress = user.BillingAddress
		order.Unconfirmed = true
		if contribution.Perk.Title == "AR-1 HOLIDAY PREORDER" {
			order.Items = []models.LineItem{
				models.LineItem{Slug_: "ar-1", Quantity: contribution.Perk.HelmetQuantity},
				models.LineItem{Slug_: "card-winter2014promo", Quantity: contribution.Perk.HelmetQuantity},
				models.LineItem{Slug_: "dogtag-winter2014promo", Quantity: contribution.Perk.HelmetQuantity},
			}
		} else {
			order.Items = []models.LineItem{
				models.LineItem{Slug_: "ar-1", Quantity: contribution.Perk.HelmetQuantity},
				models.LineItem{Slug_: "t-shirt", Quantity: contribution.Perk.GearQuantity},
				models.LineItem{Slug_: "hat", Quantity: contribution.Perk.GearQuantity},
			}
		}
		order.CreatedAt = time.Now()
		order.UpdatedAt = order.CreatedAt

		db.Put("order", order)
	}
})

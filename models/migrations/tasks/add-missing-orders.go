package tasks

import (
	"time"

	"crowdstart.io/datastore"
	"crowdstart.io/datastore/parallel"
	"crowdstart.io/models"
	"crowdstart.io/util/log"

	aeds "appengine/datastore"
)

var AddMissingOrders = parallel.Task("add-missing-orders-from-contribution", func(db *datastore.Datastore, key datastore.Key, contribution models.Contribution) {
	orders := make([]models.Order, 0)
	var keys []*aeds.Key
	var err error

	if keys, err = db.Query("order").Filter("UserId =", contribution.UserId).GetAll(db.Context, &orders); err != nil {
		log.Error("Task has encountered error: %v", err, db.Context)
		return
	}

	foundIndex := -1
	//Check to see if there is a matching order
	for i, order := range orders {
		// log.Info("items %v, %v", order.Items[0].Slug_, order.Items[0].Quantity)
		// log.Info("items %v, %v", order.Items[1].Slug_, order.Items[1].Quantity)
		// log.Info("items %v, %v", order.Items[2].Slug_, order.Items[2].Quantity)
		// log.Info("Helmet #%v, Gear #%v", contribution.Perk.HelmetQuantity, contribution.Perk.GearQuantity)

		// We have no 1 Slug contributions
		if len(order.Items) < 2 {
			break
		}
		if contribution.Perk.Title == "AR-1 HOLIDAY PREORDER" &&
			order.Items[0].Slug_ == "ar-1" &&
			order.Items[1].Slug_ == "card-winter2014promo" &&
			order.Items[2].Slug_ == "dogtag-winter2014promo" &&
			order.Items[0].Quantity == contribution.Perk.HelmetQuantity &&
			order.Items[1].Quantity == contribution.Perk.HelmetQuantity &&
			order.Items[2].Quantity == contribution.Perk.HelmetQuantity {
			foundIndex = i
		} else if contribution.Perk.Title == "SKULLY NATION GEAR" &&
			order.Items[0].Slug_ == "t-shirt" &&
			order.Items[1].Slug_ == "hat" &&
			order.Items[0].Quantity == contribution.Perk.GearQuantity &&
			order.Items[1].Quantity == contribution.Perk.GearQuantity {

		} else if order.Items[0].Slug_ == "ar-1" &&
			order.Items[1].Slug_ == "t-shirt" &&
			order.Items[2].Slug_ == "hat" &&
			order.Items[0].Quantity == contribution.Perk.HelmetQuantity &&
			order.Items[1].Quantity == contribution.Perk.GearQuantity &&
			order.Items[2].Quantity == contribution.Perk.GearQuantity {
			foundIndex = i
		}
	}
	// log.Info("orders %v, id %v", len(orders), foundIndex)

	// Upsert if found, insert new order if not found
	if foundIndex != -1 {
		// Update the email for book keeping
		orders[foundIndex].Email = contribution.Email
		db.PutKey("order", keys[foundIndex], &orders[foundIndex])
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
		} else if contribution.Perk.Title == "SKULLY NATION GEAR" {
			order.Items = []models.LineItem{
				models.LineItem{Slug_: "t-shirt", Quantity: contribution.Perk.GearQuantity},
				models.LineItem{Slug_: "hat", Quantity: contribution.Perk.GearQuantity},
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

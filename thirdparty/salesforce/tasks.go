package salesforce

import (
	"fmt"

	"appengine/delay"

	"crowdstart.io/datastore"
	"crowdstart.io/models"
	"crowdstart.io/util/log"
)

// Deferred Tasks
// This function upserts a contact into salesforce
var UpsertTask = delay.Func("SalesforceUpsert", func(client *Api, user *models.User) {
	c := client.Context

	// The email is required as it is the external ID used in salesforce
	if user.Id == "" {
		log.Panic("Id is required for upsert")
	}

	db := datastore.New(c)

	// Query out all orders (since preorder is stored as a single string)
	var orders []models.Order
	if _, err := db.Query("order").
		Filter("Email =", user.Email).
		GetAll(db.Context, &orders); err != nil {
		log.Panic("Error retrieving orders associated with the user's email", err, c)
	}

	// Query out any preorder order items and sum different skus up for totals
	items := make(map[string]int)

	for _, order := range orders {
		if order.Preorder {
			for _, item := range order.Items {
				items[item.SKU_] = items[item.SKU_] + item.Quantity
			}
		}
	}

	// Stringify
	preorders := ""

	for key, item := range items {
		preorders += fmt.Sprintf("%s: %d", key, item)
	}

	// Assign to contact and synchronize
	// contact.PreorderC = preorders

	if err := client.Push(user); err != nil {
		log.Panic("UpsertContactTask failed: %v", err, c)
	}
})

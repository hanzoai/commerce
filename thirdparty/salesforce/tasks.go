package salesforce

import (
	"appengine"

	"appengine/delay"

	"crowdstart.io/models"
	"crowdstart.io/util/log"
)

// Deferred Tasks
// UpsertTask upsert a contact into salesforce
var UpsertTask = delay.Func("SalesforceUpsert", func(c appengine.Context, campaign models.Campaign, user models.User) error {
	log.Info("Try to synchronize with salesforce", c)

	client := New(c, &campaign, true)

	//db := datastore.New(c)
	// Query out all orders (since preorder is stored as a single string)
	// var orders []models.Order
	// if _, err := db.Query("order").
	// 	Filter("Email =", user.Email).
	// 	GetAll(db.Context, &orders); err != nil {
	// 	log.Panic("Error retrieving orders associated with the user's email", err, c)
	// }

	// // Query out any preorder order items and sum different skus up for totals
	// items := make(map[string]int)

	// for _, order := range orders {
	// 	if order.Preorder {
	// 		for _, item := range order.Items {
	// 			items[item.SKU_] = items[item.SKU_] + item.Quantity
	// 		}
	// 	}
	// }

	// // Stringify
	// preorders := ""

	// for key, item := range items {
	// 	preorders += fmt.Sprintf("%s: %d", key, item)
	// }

	// // Assign to contact and synchronize
	// contact.PreorderC = preorders

	if err := client.Push(&user); err != nil {
		log.Panic("UpsertContactTask failed: %v", err, c)
	}

	return nil
})

// Wrappers to deferred function calls for type sanity
// CallUpsertTask calls the task queue delay function with the passed in params
// Values are used instead of pointers since we envoke a RPC
func CallUpsertTask(c appengine.Context, campaign *models.Campaign, user *models.User) {
	UpsertTask.Call(c, *campaign, *user)
}

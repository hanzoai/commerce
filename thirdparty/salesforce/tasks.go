package salesforce

import (
	"time"

	"appengine"

	"appengine/delay"

	"crowdstart.io/datastore"
	"crowdstart.io/models"
	"crowdstart.io/util/log"
	"crowdstart.io/util/queries"
)

// Deferred Tasks
// UpsertTask upserts a contact into salesforce
var UpsertTask = delay.Func("SalesforceUpsertTask", func(c appengine.Context, campaign models.Campaign, user models.User) {
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
		log.Panic("UpsertContactTask failed: %v", c)
	}
})

// PullUpdatedTask gets recently(20 minutes ago) updated Contact and upserts them as Users
var PullUpdatedTask = delay.Func("SalesforcePullUpdatedTask", func(c appengine.Context) {
	db := datastore.New(c)
	q := queries.New(c)

	campaign := new(models.Campaign)

	// Get user instance
	if err := db.GetKey("campaign", "dev@hanzo.ai", campaign); err != nil {
		log.Panic("Unable to get campaign from database: %v", err, c)
	}

	client := New(c, campaign, true)

	now := time.Now()

	// Get recently updated users
	users := new([]*models.User)
	// We check 15 minutes into the future in case salesforce clocks (logs based on the minute updated) is slightly out of sync with google's
	if err := client.PullUpdated(now.Add(-20*time.Minute), now, users); err != nil {
		log.Panic("Getting Updated Contacts Failed: %v, %v", err, string(client.LastBody[:]), c)
	}

	log.Info("Updating %v Users from Salesforce", len(*users), c)
	for _, user := range *users {
		if err := q.UpsertUser(user); err != nil {
			log.Panic("User '%v' could not be updated, %v", user.Id, err, c)
		} else {
			log.Info("User '%v' was successfully updated", user.Id, c)
		}
	}
})

// Wrappers to deferred function calls for type sanity
// CallUpsertTask calls the task queue delay function with the passed in params
// Values are used instead of pointers since we envoke a RPC
func CallUpsertTask(c appengine.Context, campaign *models.Campaign, user *models.User) {
	UpsertTask.Call(c, *campaign, *user)
}

// CallPullUpdatedTask calls the task queue delay function with the passed in params
func CallPullUpdatedTask(c appengine.Context) {
	PullUpdatedTask.Call(c)
}

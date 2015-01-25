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
var UpsertUserTask = delay.Func("SalesforceUpsertUserTask", func(c appengine.Context, campaign models.Campaign, user models.User) {
	log.Info("Try to synchronize with salesforce", c)

	client := New(c, &campaign, true)

	if err := client.Push(&user); err != nil {
		log.Panic("UpsertUserTask failed: %v", err, c)
	}
})

var UpsertOrderTask = delay.Func("SalesforceUpsertOrderTask", func(c appengine.Context, campaign models.Campaign, order models.Order) {

	log.Info("Try to synchronize with salesforce", c)

	client := New(c, &campaign, true)

	if err := client.Push(&order); err != nil {
		log.Panic("UpsertOrderTask failed: %v", err, c)
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
// CallUpsertUserTask calls the task queue delay function with the passed in params
// Values are used instead of pointers since we envoke a RPC
func CallUpsertUserTask(c appengine.Context, campaign *models.Campaign, user *models.User) {
	log.Info("Trying to dispatch task.", c)
	UpsertUserTask.Call(c, *campaign, *user)
	log.Info("Task dispatched.", c)
}

// CallUpsertOrderTask calls the task queue delay function with the passed in params
func CallUpsertOrderTask(c appengine.Context, campaign *models.Campaign, order *models.Order) {
	log.Info("Trying to dispatch task.", c)
	UpsertOrderTask.Call(c, *campaign, *order)
	log.Info("Task dispatched.", c)
}

// CallPullUpdatedTask calls the task queue delay function with the passed in params
func CallPullUpdatedTask(c appengine.Context) {
	log.Info("Trying to dispatch task.", c)
	PullUpdatedTask.Call(c)
	log.Info("Task dispatched.", c)
}

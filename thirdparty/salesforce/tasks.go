package salesforce

import (
	"encoding/gob"
	"errors"
	"time"

	"appengine"
	"appengine/datastore"
	"appengine/delay"

	ds "crowdstart.io/datastore"

	"crowdstart.io/models"
	"crowdstart.io/util/log"
	"crowdstart.io/util/parallel"
	"crowdstart.io/util/queries"
)

// Continuation Types for parallel library
type UserImporter struct {
	Campaign models.Campaign
}

func (ui UserImporter) NewObject() interface{} {
	return new(models.User)
}

func (ui UserImporter) Execute(c appengine.Context, key *datastore.Key, object interface{}) error {
	var ok bool
	var u *models.User
	if u, ok = object.(*models.User); !ok {
		return errors.New("Object should be of type 'user'")
	}

	client := New(c, &ui.Campaign, true)
	client.Push(u)
	return nil
}

type OrderImporter struct {
	Campaign models.Campaign
}

func (ui OrderImporter) NewObject() interface{} {
	return new(models.Order)
}

func (ui OrderImporter) Execute(c appengine.Context, key *datastore.Key, object interface{}) error {
	var ok bool
	var o *models.Order
	if o, ok = object.(*models.Order); !ok {
		return errors.New("Object should be of type 'order'")
	}

	client := New(c, &ui.Campaign, true)
	client.Push(o)
	return nil
}

// Gob registration
func init() {
	gob.Register(UserImporter{})
	gob.Register(OrderImporter{})
}

// Deferred Tasks
// UpsertUserTask upserts a contact into salesforce
var UpsertUserTask = delay.Func("SalesforceUpsertUserTask", func(c appengine.Context, campaign models.Campaign, user models.User) {
	if campaign.Salesforce.AccessToken != "" {
		log.Info("Try to synchronize with salesforce", c)

		client := New(c, &campaign, true)

		if err := client.Push(&user); err != nil {
			log.Panic("UpsertUserTask failed: %v", err, c)
		}
	}
})

// UpsertOrderTask upserts users into salesforce
var UpsertOrderTask = delay.Func("SalesforceUpsertOrderTask", func(c appengine.Context, campaign models.Campaign, order models.Order) {
	if campaign.Salesforce.AccessToken != "" {
		log.Info("Try to synchronize with salesforce", c)

		client := New(c, &campaign, true)

		if err := client.Push(&order); err != nil {
			log.Panic("UpsertOrderTask failed: %v", err, c)
		}
	}
})

// ImportUsers upserts all users into salesforce
func ImportUsers(c appengine.Context) {
	db := ds.New(c)
	campaign := models.Campaign{}

	// Get user instance
	if err := db.GetKey("campaign", "dev@hanzo.ai", &campaign); err != nil {
		log.Panic("Unable to get campaign from database: %v", err, c)
	}

	if campaign.Salesforce.AccessToken != "" {
		parallel.DatastoreJob(c, "user", 100, UserImporter{Campaign: campaign})
	}
}

// ImportOrders upserts all orders into salesforce
func ImportOrders(c appengine.Context) {
	db := ds.New(c)
	campaign := models.Campaign{}

	// Get order instance
	if err := db.GetKey("campaign", "dev@hanzo.ai", &campaign); err != nil {
		log.Panic("Unable to get campaign from database: %v", err, c)
	}

	if campaign.Salesforce.AccessToken != "" {
		parallel.DatastoreJob(c, "order", 100, OrderImporter{Campaign: campaign})
	}
}

// PullUpdatedTask gets recently(20 minutes ago) updated Contact and upserts them as Users
var PullUpdatedTask = delay.Func("SalesforcePullUpdatedTask", func(c appengine.Context) {
	db := ds.New(c)
	campaign := new(models.Campaign)

	// Get user instance
	if err := db.GetKey("campaign", "dev@hanzo.ai", campaign); err != nil {
		log.Panic("Unable to get campaign from database: %v", err, c)
	}

	if campaign.Salesforce.AccessToken != "" {
		log.Info("Try to synchronize from updated salesforce list", c)

		q := queries.New(c)
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
	}
})

// Wrappers to deferred function calls for type sanity
// CallUpsertUserTask calls the task queue delay function with the passed in params
// Values are used instead of pointers since we envoke a RPC
func CallUpsertUserTask(c appengine.Context, campaign *models.Campaign, user *models.User) {
	UpsertUserTask.Call(c, *campaign, *user)
}

// CallUpsertOrderTask calls the task queue delay function with the passed in params
func CallUpsertOrderTask(c appengine.Context, campaign *models.Campaign, order *models.Order) {
	UpsertOrderTask.Call(c, *campaign, *order)
}

// CallPullUpdatedTask calls the task queue delay function with the passed in params
func CallPullUpdatedTask(c appengine.Context) {
	PullUpdatedTask.Call(c)
}

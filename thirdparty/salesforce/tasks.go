package salesforce

import (
	"encoding/gob"
	"errors"
	"runtime/debug"
	"time"

	"appengine"
	"appengine/datastore"
	"appengine/delay"

	ds "crowdstart.io/datastore"

	"crowdstart.io/models"
	"crowdstart.io/models/migrations"
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

// Gob registration
func init() {
	gob.Register(UserImporter{})
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

// ImportOrdersTask upserts all orders into salesforce
var ImportOrdersTask = delay.Func("SalesforceImportOrdersTask", func(c appengine.Context) {
	db := ds.New(c)
	campaign := models.Campaign{}

	// Get user instance
	if err := db.GetKey("campaign", "dev@hanzo.ai", &campaign); err != nil {
		log.Panic("Unable to get campaign from database: %v", err, c)
	}

	if campaign.Salesforce.AccessToken != "" {
		var t *datastore.Iterator
		var m migrations.MigrationStatus
		var cur datastore.Cursor
		var k, mk *datastore.Key
		var err error
		var order models.Order

		name := "SalesforceImportOrdersTask"
		client := New(c, &campaign, true)

		log.Info("Try to import into salesforce", c)

		// Try to get cursor if it exists
		mk = datastore.NewKey(c, "migration", name, 0, nil)
		if err = datastore.Get(c, mk, &m); err != nil {
			log.Warn("No Preexisting Cursor found", c)
			t = datastore.NewQuery("order").Run(c)
		} else if m.Done {
			//Migration Complete
			log.Info("Import was Completed", c)
			return
		} else if cur, err = datastore.DecodeCursor(m.Cursor); err != nil {
			log.Info("Preexisting Cursor is corrupt", c)
			t = datastore.NewQuery("order").Run(c)
		} else {
			log.Info("Resuming from Preexisting Cursor", c)
			t = datastore.NewQuery("order").Start(cur).Run(c)
		}

		for {
			// Iterate Cursor
			k, err = t.Next(&order)

			if err != nil {
				// Done
				if err == datastore.Done {
					break
				}

				// Ignore field mismatch, otherwise skip record
				if err != nil {
					log.Error("Error fetching order: %v\n%v", k, err, c)
					continue
				}
			}

			// Save Migration point for resume
			mk = datastore.NewKey(c, "migration", name, 0, nil)

			if cur, err = t.Cursor(); err != nil {
				log.Warn("Could not get Cursor because %v", cur, c)
			} else {
				// It doesn't matter if cursor suceeds or not I guess
				datastore.Put(c, mk, &migrations.MigrationStatus{Cursor: cur.String(), Done: false})
			}

			log.Info("Import Key %v", k, c)
			client.Push(&order)

			debug.FreeOSMemory()
		}

		log.Info("Import Completed", c)
		datastore.Put(c, mk, &migrations.MigrationStatus{Cursor: cur.String(), Done: true})
	}
})

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

// CallImportOrdersTask calls the task queue delay function with the passed in params
func CallImportOrdersTask(c appengine.Context) {
	ImportOrdersTask.Call(c)
}

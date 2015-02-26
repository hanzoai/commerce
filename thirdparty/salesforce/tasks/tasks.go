package tasks

import (
	"time"

	"github.com/gin-gonic/gin"

	"appengine"
	"appengine/delay"

	"crowdstart.io/datastore"
	"crowdstart.io/datastore/parallel"
	"crowdstart.io/models"
	"crowdstart.io/util/log"
	"crowdstart.io/util/queries"
	"crowdstart.io/util/task"

	. "crowdstart.io/thirdparty/salesforce"
)

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

// UpsertUserTask upserts users into salesforce
var ImportUsersTask = parallel.Task("sf-import-user-task", func(db *datastore.Datastore, key datastore.Key, user models.User, campaign models.Campaign) {
	client := New(db.Context, &campaign, true)
	if err := client.Push(&user); err != nil {
		log.Debug("Error: %v", err)
	}

	// Pushes can update sync times and salesforce ids so update in datastore
	db.Put(key, &user)
})

// ImportUsers upserts all users into salesforce
func ImportUsers(c *gin.Context) {
	db := datastore.New(c)
	campaign := models.Campaign{}

	// Get user instance
	if err := db.GetKind("campaign", "dev@hanzo.ai", &campaign); err != nil {
		log.Panic("Unable to get campaign from database: %v", err, c)
	}

	if campaign.Salesforce.AccessToken != "" {
		parallel.Run(c, "user", 100, ImportUsersTask, campaign)
	}
}

// UpsertOrderTask upserts users into salesforce
var ImportOrdersTask = parallel.Task("sf-import-order-task", func(db *datastore.Datastore, key datastore.Key, order models.Order, campaign models.Campaign) {
	client := New(db.Context, &campaign, true)
	if err := client.Push(&order); err != nil {
		log.Debug("Error: %v, '%v'", err, order.UserId)
	}

	// Pushes can update sync times and salesforce ids so update in datastore
	db.Put(key, &order)
})

// ImportOrders upserts all orders into salesforce
func ImportOrders(c *gin.Context) {
	db := datastore.New(c)
	campaign := models.Campaign{}

	// Get order instance
	if err := db.GetKind("campaign", "dev@hanzo.ai", &campaign); err != nil {
		log.Panic("Unable to get campaign from database: %v", err, c)
	}

	if campaign.Salesforce.AccessToken != "" {
		parallel.Run(c, "order", 100, ImportOrdersTask, campaign)
	}
}

// UpsertOrderTask upserts users into salesforce
var ImportProductVariantsTask = parallel.Task("sf-import-product-task", func(db *datastore.Datastore, key datastore.Key, variant models.ProductVariant, campaign models.Campaign) {
	client := New(db.Context, &campaign, true)
	if err := client.Push(&variant); err != nil {
		log.Error("Unable to update variant '%v': %v", variant.Id, err, db.Context)
	}

	// Pushes can update sync times and salesforce ids so update in datastore
	db.Put(key, &variant)
})

// ImportOrders upserts all orders into salesforce
func ImportProductVariant(c *gin.Context) {
	db := datastore.New(c)
	var campaign models.Campaign

	// Get order instance
	if err := db.GetKind("campaign", "dev@hanzo.ai", &campaign); err != nil {
		log.Panic("Unable to get campaign from database: %v", err, c)
	}

	if campaign.Salesforce.AccessToken == "" {
		log.Panic("Missing Salesforce Access Token: %#v", campaign.Salesforce, c)
	}

	parallel.Run(c, "variant", 100, ImportProductVariantsTask, campaign)
}

// PullUpdatedTask gets recently(20 minutes ago) updated Contact and upserts them as Users
var PullUpdatedTask = delay.Func("SalesforcePullUpdatedTask", func(c appengine.Context) {
	db := datastore.New(c)
	campaign := new(models.Campaign)

	// Get user instance
	if err := db.GetKind("campaign", "dev@hanzo.ai", campaign); err != nil {
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

// PullUpdatedTask gets recently(20 minutes ago) updated Contact and upserts them as Users
var PullUpdatedSinceCleanUpTask = delay.Func("SalesforcePullUpdatedSinceCleanUpTask", func(c appengine.Context) {
	db := datastore.New(c)
	campaign := new(models.Campaign)

	// Get user instance
	if err := db.GetKind("campaign", "dev@hanzo.ai", campaign); err != nil {
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
		if err := client.PullUpdated(now.Add(-22*24*time.Hour), now, users); err != nil {
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

func init() {
	task.Register("salesforce-sync-users", ImportUsers)
	task.Register("salesforce-sync-orders", ImportOrders)
	task.Register("salesforce-sync-product-variants", ImportProductVariant)
	task.Register("salesforce-sync-updated", CallPullUpdatedTask)
	task.Register("salesforce-sync-updated-since-cleanup", PullUpdatedSinceCleanUpTask)
}

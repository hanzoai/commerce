package tasks

import (
	"github.com/gin-gonic/gin"

	"appengine"
	"appengine/delay"

	"hanzo.io/datastore"
	"hanzo.io/datastore/parallel"
	// "hanzo.io/models"

	// "hanzo.io/util/queries"
	"hanzo.io/util/task"

	"hanzo.io/models/campaign"
	"hanzo.io/models/order"
	"hanzo.io/models/user"
	"hanzo.io/models/variant"
	// . "hanzo.io/thirdparty/salesforce"
)

// Deferred Tasks
// UpsertUserTask upserts a contact into salesforce
var UpsertUserTask = delay.Func("SalesforceUpsertUserTask", func(c appengine.Context, campaign *campaign.Campaign, user *user.User) {
	// if campaign.Salesforce.AccessToken != "" {
	// 	log.Info("Try to synchronize with salesforce", c)

	// 	client := New(c, &campaign, true)

	// 	if err := client.Push(user); err != nil {
	// 		log.Panic("UpsertUserTask failed: %v", err, c)
	// 	}
	// }
})

// UpsertOrderTask upserts users into salesforce
var UpsertOrderTask = delay.Func("SalesforceUpsertOrderTask", func(c appengine.Context, campaign *campaign.Campaign, order *order.Order) {
	// if campaign.Salesforce.AccessToken != "" {
	// 	log.Info("Try to synchronize with salesforce", c)

	// 	client := New(c, &campaign, true)

	// 	if err := client.Push(order); err != nil {
	// 		log.Panic("UpsertOrderTask failed: %v", err, c)
	// 	}
	// }
})

// UpsertUserTask upserts users into salesforce
var ImportUsersTask = parallel.New("sf-import-user-task", func(db *datastore.Datastore, user *user.User, campaign *campaign.Campaign) {
	// client := New(db.Context, &campaign, true)
	// if err := client.Push(user); err != nil {
	// 	log.Debug("Error: %v", err)
	// }

	// // Pushes can update sync times and salesforce ids so update in datastore
	// db.Put(key, &user)
})

// ImportUsers upserts all users into salesforce
func ImportUsers(c *gin.Context) {
	// db := datastore.New(c)
	// campaign := campaign.Campaign{}

	// Get user instance
	// if err := db.GetKind("campaign", "dev@hanzo.ai", &campaign); err != nil {
	// 	log.Panic("Unable to get campaign from database: %v", err, c)
	// }

	// if campaign.Salesforce.AccessToken != "" {
	// 	ImportUsersTask.Run(c, 100, campaign)
	// }
}

// UpsertMissingUserTask upserts users not synchronized into salesforce
var ImportMissingUsersTask = parallel.New("sf-import-missing-user-task", func(db *datastore.Datastore, user *user.User, campaign campaign.Campaign) {
	// // Skip users with missing
	// if user.SalesforceId() != "" {
	// 	return
	// }

	// client := New(db.Context, &campaign, true)
	// if err := client.Push(user); err != nil {
	// 	log.Debug("Error: %v", err)
	// }

	// // Pushes can update sync times and salesforce ids so update in datastore
	// db.Put(key, &user)
})

// ImportMissingUsers upserts all users not synchronized into salesforce
func ImportMissingUsers(c *gin.Context) {
	// db := datastore.New(c)
	// campaign := campaign.Campaign{}

	// Get user instance
	// if err := db.GetKind("campaign", "dev@hanzo.ai", &campaign); err != nil {
	// 	log.Panic("Unable to get campaign from database: %v", err, c)
	// }

	// if campaign.Salesforce.AccessToken != "" {
	// 	ImportMissingUsersTask.Run(c, 100, campaign)
	// }
}

// UpsertOrderTask upserts orders into salesforce
var ImportOrdersTask = parallel.New("sf-import-order-task", func(db *datastore.Datastore, order *order.Order, campaign campaign.Campaign) {
	// client := New(db.Context, &campaign, true)
	// if err := client.Push(order); err != nil {
	// 	log.Debug("Error: %v, '%v'", err, order.UserId)
	// }

	// // Pushes can update sync times and salesforce ids so update in datastore
	// db.Put(key, &order)
})

// ImportOrders upserts all orders into salesforce
func ImportOrders(c *gin.Context) {
	// db := datastore.New(c)
	// campaign := campaign.Campaign{}

	// Get order instance
	// if err := db.GetKind("campaign", "dev@hanzo.ai", &campaign); err != nil {
	// 	log.Panic("Unable to get campaign from database: %v", err, c)
	// }

	// if campaign.Salesforce.AccessToken != "" {
	// 	ImportOrdersTask.Run(c, 100, campaign)
	// }
}

// UpsertMissingOrderTask upserts orders not synchronized into salesforce
var ImportMissingOrdersTask = parallel.New("sf-import-missing-order-task", func(db *datastore.Datastore, order *order.Order, campaign campaign.Campaign) {
	// // Skip orders with missing
	// if order.SalesforceId() != "" {
	// 	return
	// }

	// client := New(db.Context, &campaign, true)
	// if err := client.Push(order); err != nil {
	// 	log.Debug("Error: %v, '%v'", err, order.UserId)
	// }

	// // Pushes can update sync times and salesforce ids so update in datastore
	// db.Put(key, &order)
})

// ImportMissingOrders upserts all orders not synchronized into salesforce
func ImportMissingOrders(c *gin.Context) {
	// db := datastore.New(c)
	// campaign := campaign.Campaign{}

	// Get order instance
	// if err := db.GetKind("campaign", "dev@hanzo.ai", &campaign); err != nil {
	// 	log.Panic("Unable to get campaign from database: %v", err, c)
	// }

	// if campaign.Salesforce.AccessToken != "" {
	// 	ImportMissingOrdersTask.Run(c, 100, campaign)
	// }
}

// UpsertOrderTask upserts users into salesforce
var ImportProductVariantsTask = parallel.New("sf-import-product-task", func(db *datastore.Datastore, variant *variant.Variant, campaign campaign.Campaign) {
	// client := New(db.Context, &campaign, true)
	// if err := client.Push(variant); err != nil {
	// 	log.Error("Unable to update variant '%v': %v", variant.Id, err, db.Context)
	// }

	// // Pushes can update sync times and salesforce ids so update in datastore
	// db.Put(key, &variant)
})

// ImportOrders upserts all orders into salesforce
func ImportProductVariant(c *gin.Context) {
	// db := datastore.New(c)
	// var campaign models.Campaign

	// // Get order instance
	// if err := db.GetKind("campaign", "dev@hanzo.ai", &campaign); err != nil {
	// 	log.Panic("Unable to get campaign from database: %v", err, c)
	// }

	// if campaign.Salesforce.AccessToken == "" {
	// 	log.Panic("Missing Salesforce Access Token: %#v", campaign.Salesforce, c)
	// }

	// ImportProductVariantsTask.Run(c, 100, campaign)
}

// PullUpdatedTask gets recently(20 minutes ago) updated Contact and upserts them as Users
var PullUpdatedUsersTask = delay.Func("SalesforcePullUpdatedUsersTask", func(c appengine.Context) {
	// db := datastore.New(c)
	// campaign := new(models.Campaign)

	// // Get user instance
	// if err := db.GetKind("campaign", "dev@hanzo.ai", campaign); err != nil {
	// 	log.Panic("Unable to get campaign from database: %v", err, c)
	// }

	// if campaign.Salesforce.AccessToken != "" {
	// 	log.Info("Try to synchronize from updated salesforce list", c)

	// 	// q := queries.New(c)
	// 	// client := New(c, campaign, true)

	// 	// now := time.Now()

	// 	// // Get recently updated users
	// 	// users := new([]*models.User)
	// 	// // We check 15 minutes into the future in case salesforce clocks (logs based on the minute updated) is slightly out of sync with google's
	// 	// if err := client.PullUpdated(now.Add(-21*time.Minute), now, users); err != nil {
	// 	// 	log.Panic("Getting Updated Contacts Failed: %v, %v", err, string(client.LastBody[:]), c)
	// 	// }

	// 	// log.Info("Updating %v Users from Salesforce", len(*users), c)
	// 	// for _, user := range *users {
	// 	// 	if err := q.UpsertUser(user); err != nil {
	// 	// 		log.Panic("User '%v' could not be updated, %v", user.Id, err, c)
	// 	// 	} else {
	// 	// 		log.Info("User '%v' was successfully updated", user.Id, c)
	// 	// 	}
	// 	// }
	// }
})

// PullUpdatedTask gets recently(20 minutes ago) updated Contact and upserts them as Orders
var PullUpdatedOrdersTask = delay.Func("SalesforcePullUpdatedOrderTask", func(c appengine.Context) {
	// db := datastore.New(c)
	// campaign := new(models.Campaign)

	// // Get user instance
	// if err := db.GetKind("campaign", "dev@hanzo.ai", campaign); err != nil {
	// 	log.Panic("Unable to get campaign from database: %v", err, c)
	// }

	// if campaign.Salesforce.AccessToken != "" {
	// 	log.Info("Try to synchronize from updated salesforce list", c)

	// 	client := New(c, campaign, true)

	// 	now := time.Now()

	// 	// Get recently updated orders
	// 	orders := new([]*models.Order)
	// 	// We check 15 minutes into the future in case salesforce clocks (logs based on the minute updated) is slightly out of sync with google's
	// 	if err := client.PullUpdated(now.Add(-21*time.Minute), now, orders); err != nil {
	// 		log.Error("Getting Updated Contacts Failed: %v, %v", err, string(client.LastBody[:]), c)
	// 	}

	// 	log.Info("Updating %v Orders from Salesforce", len(*orders), c)
	// 	for _, order := range *orders {
	// 		key, _ := db.DecodeKey(order.Id)
	// 		if _, err := db.Put(key, order); err != nil {
	// 			log.Error("Order '%v' could not be updated, %v", order.Id, err, c)
	// 		} else {
	// 			log.Info("Order '%v' was successfully updated", order.Id, c)
	// 		}
	// 	}
	// }
})

// PullUpdatedTask gets recently(20 minutes ago) updated Contact and upserts them as Users
var PullUpdatedSinceCleanUpTask = delay.Func("SalesforcePullUpdatedSinceCleanUpTask", func(c appengine.Context) {
	// db := datastore.New(c)
	// campaign := new(models.Campaign)

	// // Get user instance
	// if err := db.GetKind("campaign", "dev@hanzo.ai", campaign); err != nil {
	// 	log.Error("Unable to get campaign from database: %v", err, c)
	// }

	// if campaign.Salesforce.AccessToken != "" {
	// 	log.Info("Try to synchronize from updated salesforce list", c)

	// 	q := queries.New(c)
	// 	client := New(c, campaign, true)

	// 	now := time.Now()

	// 	// Get recently updated users
	// 	users := new([]*models.User)
	// 	// We check 15 minutes into the future in case salesforce clocks (logs based on the minute updated) is slightly out of sync with google's
	// 	if err := client.PullUpdated(now.Add(-22*24*time.Hour), now, users); err != nil {
	// 		log.Error("Getting Updated Contacts Failed: %v, %v", err, string(client.LastBody[:]), c)
	// 	}

	// 	log.Info("Updating %v Users from Salesforce", len(*users), c)
	// 	for _, user := range *users {
	// 		if err := q.UpsertUser(user); err != nil {
	// 			log.Error("User '%v' could not be updated, %v", user.Id, err, c)
	// 		} else {
	// 			log.Info("User '%v' was successfully updated", user.Id, c)
	// 		}
	// 	}
	// }
})

// Wrappers to deferred function calls for type sanity
// CallUpsertUserTask calls the task queue delay function with the passed in params
// Values are used instead of pointers since we envoke a RPC
func CallUpsertUserTask(c appengine.Context, campaign *campaign.Campaign, user *user.User) {
	UpsertUserTask.Call(c, campaign, user)
}

// CallUpsertOrderTask calls the task queue delay function with the passed in params
func CallUpsertOrderTask(c appengine.Context, campaign *campaign.Campaign, order *order.Order) {
	UpsertOrderTask.Call(c, campaign, order)
}

// Get Salesforce Ids for every user

// PopulateMissingUserSFIdsTask adds all missing salesforce ids for users
var PopulateMissingUserSFIdsTask = parallel.New("sf-populate-user-ids", func(db *datastore.Datastore, usr *user.User, campaign campaign.Campaign) {
	// if usr.SalesforceId() != "" {
	// 	return
	// }

	// client := New(db.Context, &campaign, true)
	// blankUser := user.New(db)
	// if err := client.Pull(usr.Id(), blankUser); err != nil {
	// 	log.Debug("Error: %v", err)
	// }

	// usr.SetSalesforceId(blankUser.SalesforceId())
	// usr.SetSalesforceId2(blankUser.SalesforceId2())

	// // Pushes can update sync times and salesforce ids so update in datastore
	// db.Put(key, usr)
})

// PopulateMissingUserSFIds ensures all users have salesforce ids
func PopulateMissingUserSFIds(c *gin.Context) {
	// db := datastore.New(c)
	// campaign := campaign.Campaign{}

	// Get user instance
	// if err := db.GetKind("campaign", "dev@hanzo.ai", &campaign); err != nil {
	// 	log.Panic("Unable to get campaign from database: %v", err, c)
	// }

	// if campaign.Salesforce.AccessToken != "" {
	// 	PopulateMissingUserSFIdsTask.Run(c, 100, campaign)
	// }
}

func init() {
	task.New("salesforce-sync-users", ImportUsers)
	task.New("salesforce-sync-orders", ImportOrders)
	task.New("salesforce-sync-missing-users", ImportMissingUsers)
	task.New("salesforce-sync-missing-orders", ImportMissingOrders)
	task.New("salesforce-sync-product-variants", ImportProductVariant)
	task.New("salesforce-sync-updated-users", PullUpdatedUsersTask)
	task.New("salesforce-sync-updated-orders", PullUpdatedOrdersTask)
	task.New("salesforce-sync-updated-since-cleanup", PullUpdatedSinceCleanUpTask)
	task.New("salesforce-populate-missing-user-sf-ids", PopulateMissingUserSFIds)
}

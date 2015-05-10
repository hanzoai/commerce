package migrations

import (
	"log"

	"github.com/gin-gonic/gin"

	"crowdstart.io/datastore"
	"crowdstart.io/datastore/parallel"
	"crowdstart.io/models"
	"crowdstart.io/models/migrations/tasks"
	"crowdstart.io/util/task"
)

// Define all migrations
func init() {
	// Add email to orders
	task.Register("migrations-add-email-to-orders", addEmailToOrders)

	// Add email back to contribution
	task.Register("migrations-add-email-to-contribution", func(c *gin.Context) {
		parallel.Run(c, "contribution", 100, tasks.AddEmailToContribution)
	})

	// Add missing orders for each contributors
	task.Register("migrations-add-missing-orders", func(c *gin.Context) {
		parallel.Run(c, "contribution", 50, tasks.AddMissingOrders)
	})

	// Add missing orders for each contributors
	task.Register("migrations-add-estimated-delivery-to-order", func(c *gin.Context) {
		parallel.Run(c, "order", 50, tasks.AddEstimateDeliveryToOrder)
	})

	// Add missing orders for each contributors
	task.Register("migrations-set-estimated-delivery-date-to-late-2015", func(c *gin.Context) {
		parallel.Run(c, "order", 50, tasks.SetEstimatedDeliveryDateToLate2015)
	})

	// Add missing orders for each contributors
	task.Register("migrations-fix-perk-420", func(c *gin.Context) {
		parallel.Run(c, "order", 50, tasks.FixPerk420)
	})

	// Add missing orders for each contributors
	task.Register("migrations-fix-estimated-delivery-420", func(c *gin.Context) {
		parallel.Run(c, "order", 50, tasks.FixEstimatedDelivery420)
	})

	// Add missing orders for each contributors
	task.Register("migrations-add-id-to-order", func(c *gin.Context) {
		parallel.Run(c, "order", 50, tasks.AddIdToOrder)
	})

	// Create a Entity set of all broken orders
	task.Register("migrations-list-broken-orders", listBrokenOrders)

	// Misc clean up
	task.Register("migrations-fix-email", fixEmail)

	// Add missing orders for each contributors
	task.Register("migrations-fix-indiegogo-order-price", func(c *gin.Context) {
		parallel.Run(c, "contribution", 50, tasks.FixOrderPrice)
	})

	// Add missing orders for each contributors
	task.Register("migrations-generate-new-id-for-unsynced-orders", func(c *gin.Context) {
		parallel.Run(c, "order", 50, tasks.GenerateNewIdForUnsyncedOrders)
	})

	// Add missing orders for each contributors
	task.Register("migrations-generate-new-userid-for-unsynced-orders", func(c *gin.Context) {
		parallel.Run(c, "order", 50, tasks.GenerateNewUserIdForUnsyncedOrders)
	})

	// Update orders with defaults
	task.Register("migrations-add-skus-to-order", func(c *gin.Context) {
		parallel.Run(c, "order", 50, tasks.AddSkusToOrder)
	})

	// Add missing orders for each contributors
	task.Register("migrations-fix-preorder-corruption", func(c *gin.Context) {
		db := datastore.New(c)
		campaign := models.Campaign{}

		if err := db.GetKind("campaign", "dev@hanzo.ai", &campaign); err != nil {
			log.Panic("Unable to get campaign from database: %v", err, c)
		}

		if campaign.Salesforce.AccessToken != "" {
			parallel.Run(c, "order", 50, tasks.FixPreorderCorruption, campaign)
		}
	})
}

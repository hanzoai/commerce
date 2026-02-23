package workflows

import (
	"github.com/hanzoai/commerce/billing/engine"
	"go.temporal.io/sdk/worker"
)

// RegisterWorkflows registers all billing workflows and activities
// with the provided Temporal worker.
func RegisterWorkflows(w worker.Worker, burnCredits engine.CreditBurner) {
	// Register workflows
	w.RegisterWorkflow(SubscriptionLifecycleWorkflow)
	w.RegisterWorkflow(DunningWorkflow)

	// Register activities
	activities := &BillingActivities{
		BurnCredits: burnCredits,
	}
	w.RegisterActivity(activities)
}

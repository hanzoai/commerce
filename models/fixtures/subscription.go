package fixtures

import (
	"time"

	"github.com/hanzoai/commerce/billing/trial"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/zap-proto/zip"
)

// ProSubscription gives the fixture org a current pro subscription, so the
// billing paywall admits it.
//
// billing/paywall gates the admin APIs behind one of three unlocks — a pro
// subscription, a live trial credit, or a redeemed invite — and correctly
// offers no test bypass. Without one of them every API suite gets 402
// subscription_required, which is the paywall working, not a bug in the suite.
//
// A subscription is the unlock chosen here because it is the state a paying
// customer is actually in: trial credit expires and an invite is a one-off, so
// either would make the suites model an edge case while claiming to exercise
// the ordinary path.
//
// Status Active with PeriodEnd in the future is what paywall.hasSubscription
// looks for, and PlanId must be trial.PlanSlug — the gate matches on the plan
// slug, so a subscription to any other plan does not unlock anything.
var ProSubscription = New("pro-subscription", func(c *zip.Ctx) *subscription.Subscription {
	db := getNamespaceDb(c)

	now := time.Now()

	sub := subscription.New(db)
	sub.GetOrCreate("PlanId=", trial.PlanSlug)
	sub.PlanId = trial.PlanSlug
	sub.Status = subscription.Active
	sub.PeriodStart = now.Add(-Month)
	sub.PeriodEnd = now.Add(Month)

	sub.MustPut()

	return sub
})

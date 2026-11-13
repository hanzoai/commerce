package fixtures

import (
	"time"

	"github.com/hanzoai/commerce/models/subscription"
	"github.com/zap-proto/zip"
)

// ProSubscription gives the fixture org a current paid subscription, so the
// billing paywall admits it.
//
// billing/paywall gates the admin APIs behind a paid plan or a redeemed invite,
// and correctly offers no test bypass. Without one of them every API suite gets
// 402 subscription_required, which is the paywall working, not a bug in the suite.
//
// A paid subscription is the unlock chosen here because it is the state a paying
// customer is actually in: an invite is a one-off, so it would make the suites
// model an edge case while claiming to exercise the ordinary path.
//
// The row is what billing.Unlocks looks for: Active, PeriodEnd in the future, on
// a paid plan of the catalog (go), and payment-backed (a card processor manages
// it).
var ProSubscription = New("pro-subscription", func(c *zip.Ctx) *subscription.Subscription {
	db := getNamespaceDb(c)

	now := time.Now()

	sub := subscription.New(db)
	sub.GetOrCreate("PlanId=", "go")
	sub.PlanId = "go"
	sub.Plan.Slug, sub.Plan.Name, sub.Plan.Category, sub.Plan.Price = "go", "Go", "personal", 800
	sub.ProviderType = "square"
	sub.Status = subscription.Active
	sub.PeriodStart = now.Add(-Month)
	sub.PeriodEnd = now.Add(Month)

	sub.MustPut()

	return sub
})

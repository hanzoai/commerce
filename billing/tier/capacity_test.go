package tier_test

// A tier does not hold a CAPACITY, and this is the gate that keeps it so.
//
// The registry used to carry MaxAgents beside the catalog's `limits.agents`, and
// the two were one fact in two homes with no stated precedence — worse, they
// could not have agreed even in principle: this registry is keyed by TIER NAME
// and the catalog by PLAN SLUG, and six slugs collapse onto four names, so `dev`,
// `max` and `team` all resolved to Pro and `max`'s single bot had no field to
// land in at all.
//
// api/billing.TierLimits composes the two now — the tier says which models, the
// catalog says how many agents — and this test walks the struct so that adding a
// count back here is a red build rather than a silent second answer.

import (
	"reflect"
	"strings"
	"testing"

	"github.com/hanzoai/commerce/billing/tier"
)

// TestTheRegistryDoesNotHoldACapacity fails on any field of tier.Config whose
// name reads as a roster count.
//
// MUTATION: add `MaxAgents int` back to Config and this names it.
func TestTheRegistryDoesNotHoldACapacity(t *testing.T) {
	rt := reflect.TypeOf(tier.Config{})
	for i := range rt.NumField() {
		name := rt.Field(i).Name
		low := strings.ToLower(name)
		if strings.Contains(low, "agent") || strings.Contains(low, "bot") || strings.Contains(low, "seat") {
			t.Fatalf("tier.Config.%s is a CAPACITY, and capacity is published per PLAN "+
				"in the catalog (subscription.json limits.agents/limits.bots, read through "+
				"api/billing.AgentsIncluded). A tier is keyed by name and a plan by slug; "+
				"six slugs collapse onto four names, so this field cannot agree with the "+
				"catalog even when both are correct. Compose it in api/billing.TierLimits.", name)
		}
	}
}

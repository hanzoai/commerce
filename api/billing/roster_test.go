package billing

// The plan's ROSTER — how many agents and bots a customer may run — has ONE home,
// the catalog, and this is the gate that keeps the wire agreeing with it.
//
// It used to have two. billing/tier's registry carried MaxAgents beside
// subscription.json's `limits.agents`, keyed by TIER NAME where the catalog is
// keyed by PLAN SLUG, so `dev`, `max` and `team` all collapsed onto Pro and
// `max`'s one bot had no field to land in. TierLimits composes the two now and
// these tests walk every plan the catalog ships, so a slug added tomorrow is
// covered without anybody remembering this file.

import (
	"testing"

	"github.com/hanzoai/commerce/billing/tier"
)

// TestTheWireReportsTheCatalogsRoster: for every plan the catalog sells, what
// TierLimits publishes is what the catalog published, under the wire's own
// reading of zero.
//
// MUTATION: return the registry's old numbers from tierLimits and this fails
// naming the first slug where a tier name and a plan slug disagree.
func TestTheWireReportsTheCatalogsRoster(t *testing.T) {
	cfg := tier.Get(tier.Pro) // the tier half is irrelevant to the roster; any will do
	seen := 0
	for _, p := range catalog {
		agents, agentsKnown := AgentsIncluded(p.Slug)
		bots, botsKnown := BotsIncluded(p.Slug)
		if !agentsKnown && !botsKnown {
			continue // a plan that publishes no roster is not what this measures
		}
		seen++
		lim := tierLimits(cfg, p.Slug)
		if got, want := lim.MaxAgents, capacity(agents, agentsKnown); got != want {
			t.Errorf("plan %q: the wire says maxAgents %d, the catalog says %d", p.Slug, got, want)
		}
		if got, want := lim.MaxBots, capacity(bots, botsKnown); got != want {
			t.Errorf("plan %q: the wire says maxBots %d, the catalog says %d", p.Slug, got, want)
		}
		if lim.UnlimitedAgents != (lim.MaxAgents == 0) {
			t.Errorf("plan %q: unlimitedAgents %v disagrees with maxAgents %d",
				p.Slug, lim.UnlimitedAgents, lim.MaxAgents)
		}
	}
	if seen == 0 {
		t.Fatal("no plan in the catalog publishes a roster — this test measured nothing, " +
			"which is how a gate passes over an empty set")
	}
	t.Logf("checked the roster of %d plans", seen)
}

// TestTheWireKeepsItsUnlimitedReading pins the translation between the catalog's
// three answers and the wire's two. It is the one place the two vocabularies
// meet, so it is the one place the mapping is written down.
//
// MUTATION: return n unchanged from capacity and the unlimited row fails with
// -1, which every reader comparing `count >= max` would treat as "refuse
// everything".
func TestTheWireKeepsItsUnlimitedReading(t *testing.T) {
	for _, tc := range []struct {
		name  string
		n     int
		known bool
		want  int
	}{
		{"the catalog says unlimited", -1, true, 0},
		{"the catalog says a real bound", 10, true, 10},
		{"the catalog says one", 1, true, 1},
		{"the catalog is silent, so serve without a bound", 0, false, 0},
		{"the catalog says none, which this wire cannot distinguish from unlimited", 0, true, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := capacity(tc.n, tc.known); got != tc.want {
				t.Fatalf("capacity(%d, %v) = %d, want %d", tc.n, tc.known, got, tc.want)
			}
		})
	}
}

// TestSilenceAdmits is the catalog's own rule, restated where it is enforced: a
// missing capacity has no safe number, and zero would refuse a customer their
// first agent. A slug nobody has ever published must serve without a bound.
func TestSilenceAdmits(t *testing.T) {
	lim := tierLimits(tier.Get(tier.Free), "a-plan-that-does-not-exist")
	if lim.MaxAgents != 0 || !lim.UnlimitedAgents {
		t.Fatalf("an unknown plan was given a ceiling: maxAgents %d, unlimited %v",
			lim.MaxAgents, lim.UnlimitedAgents)
	}
}

package billing

import (
	"net/http"
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/util/test/ae"
)

// A plan move is scored by list price, never by an AI allowance the catalog no
// longer states. dev and max-20x carry no usage figure in @hanzo/plans 1.8.16, so
// a score built from one reads both as equal and waves the upgrade through; the
// price still says max-20x is ten times dev.
func TestPlanMoveScoresListPrice(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("acme")
	db := datastore.New(org.Namespaced(ctx))

	sub := func(user, slug string) *subscription.Subscription {
		s := subscription.New(db)
		s.UserId = user
		s.Status = subscription.Active
		s.ProviderType = "square"
		s.Plan.Slug = slug
		s.PlanId = slug
		if err := s.Create(); err != nil {
			t.Fatalf("seed %s on %s: %v", user, slug, err)
		}
		return s
	}
	slugOf := func(s *subscription.Subscription) string {
		got := subscription.New(db)
		if err := got.GetById(s.Id()); err != nil {
			t.Fatalf("reload %s: %v", s.Id(), err)
		}
		return got.Plan.Slug
	}

	up := sub("acme/up", "dev")
	if r := invokeSubPatch(org, ctx, c1OrgAdmin, up.Id(), `{"planId":"max-20x"}`); r.StatusCode != http.StatusForbidden {
		t.Fatalf("org admin PATCH dev→max-20x: status=%d body=%s, want 403", r.StatusCode, bodyOf(r))
	}
	if got := slugOf(up); got != "dev" {
		t.Fatalf("refused upgrade left the subscription on %q, want dev", got)
	}

	down := sub("acme/down", "max-20x")
	if r := invokeSubPatch(org, ctx, c1OrgAdmin, down.Id(), `{"planId":"dev"}`); r.StatusCode != http.StatusOK {
		t.Fatalf("org admin PATCH max-20x→dev: status=%d body=%s, want 200", r.StatusCode, bodyOf(r))
	}
	if got := slugOf(down); got != "dev" {
		t.Fatalf("downgrade left the subscription on %q, want dev", got)
	}
}

package billing

import (
	"net/http"
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/util/test/ae"
)

// A plan move is scored by list price, never by an AI allowance the catalog no
// longer states. go and max-15x carry no usage figure in @hanzo/plans, so a score
// built from one reads both as equal and waves the upgrade through; the price
// still says max-15x is twelve times go.
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

	up := sub("acme/up", "go")
	if r := invokeSubPatch(org, ctx, c1OrgAdmin, up.Id(), `{"planId":"max-15x"}`); r.StatusCode != http.StatusForbidden {
		t.Fatalf("org admin PATCH go→max-15x: status=%d body=%s, want 403", r.StatusCode, bodyOf(r))
	}
	if got := slugOf(up); got != "go" {
		t.Fatalf("refused upgrade left the subscription on %q, want go", got)
	}

	down := sub("acme/down", "max-15x")
	if r := invokeSubPatch(org, ctx, c1OrgAdmin, down.Id(), `{"planId":"go"}`); r.StatusCode != http.StatusOK {
		t.Fatalf("org admin PATCH max-15x→go: status=%d body=%s, want 200", r.StatusCode, bodyOf(r))
	}
	if got := slugOf(down); got != "go" {
		t.Fatalf("downgrade left the subscription on %q, want go", got)
	}
}

// A price staff agreed with a customer is for the plan it was agreed for
// (customerprice is keyed by subject and slug): a move onto another plan bills the
// price agreed for that plan, or its list price. Ultra lists above Max, so an org
// admin moves an Ultra subscriber onto Max unaided; carried along, Ultra's agreed
// $50 would bill Max at $50. Mutation: drop the CustomerCents assignment in the
// PATCH move and the list case renews at 5000.
func TestAPlanMoveBillsThePriceAgreedForThePlanMovedOnto(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	if _, _, err := SeedPlans(ctx); err != nil {
		t.Fatalf("seed: %v", err)
	}
	org := moneyOrg("acme")
	db := datastore.New(org.Namespaced(ctx))
	withFakeSquare(t, squareMock("cust_mv", "ccof_mv", "sqpay_mv"))

	for _, tc := range []struct {
		subject       string
		agreed, wants int64
	}{
		{"acme/list", 0, lookupPlan("max-15x").Price},
		{"acme/agreed", 7000, 7000},
	} {
		s := seedCardBackedSub(t, db, tc.subject, "ultra", "ccof_mv", "cust_mv")
		if _, err := SetCustomerPrice(ctx, org, PriceIn{Subject: tc.subject, Plan: "ultra", Cents: 5000, Note: "agreed for ultra"}); err != nil {
			t.Fatalf("%s: agree ultra: %v", tc.subject, err)
		}
		if reload(t, db, s.Id()).CustomerCents != 5000 {
			t.Fatalf("%s: the ultra row does not carry its agreed price", tc.subject)
		}
		if tc.agreed > 0 {
			if _, err := SetCustomerPrice(ctx, org, PriceIn{Subject: tc.subject, Plan: "max-15x", Cents: tc.agreed, Note: "agreed for max"}); err != nil {
				t.Fatalf("%s: agree max-15x: %v", tc.subject, err)
			}
		}
		if r := invokeSubPatch(org, ctx, c1OrgAdmin, s.Id(), `{"planId":"max-15x"}`); r.StatusCode != http.StatusOK {
			t.Fatalf("%s: org admin PATCH ultra→max-15x: status=%d body=%s, want 200", tc.subject, r.StatusCode, bodyOf(r))
		}
		moved := reload(t, db, s.Id())
		if moved.Plan.Slug != "max-15x" || moved.CustomerCents != tc.agreed {
			t.Fatalf("%s: after the move the row holds %q at an agreed %d, want max-15x at %d", tc.subject, moved.Plan.Slug, moved.CustomerCents, tc.agreed)
		}
		due(t, db, s.Id())
		if out := renewDue(ctx, org, tc.subject); len(out) != 1 || !out[0].Success || out[0].AmountCents != tc.wants {
			t.Fatalf("%s: renewal = %+v, want %d on the card", tc.subject, out, tc.wants)
		}
	}
}

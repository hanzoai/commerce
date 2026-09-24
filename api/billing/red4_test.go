package billing

import (
	"context"
	"errors"
	"testing"
	"time"

	squarecore "github.com/square/square-go-sdk/v3/core"

	"github.com/hanzoai/commerce/billing/engine"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/util/test/ae"
)

// An ecosystem org's own plan bought with credits (a paid "credit" first
// invoice) is comped. The first live cycle past its period moves it on, onto a
// period that starts where its paid first invoice ends — which is Realign's
// rule — so every live cycle after it is refused until an operator realigns,
// and the realignment moves it back into the past.
func TestRed4_CompedRowBlocksEveryLiveCycle(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	eco := moneyOrg("hanzo")
	withFakeSquare(t, squareMock("", "", "sqpay_eco"))
	db := datastore.New(eco.Namespaced(ctx))
	now := time.Now()
	p, err := resolveSubscriptionPlan(db, "dev")
	if err != nil {
		t.Fatal(err)
	}
	sub := subscription.New(db)
	sub.UserId = "hanzo"
	sub.Quantity = 1
	engine.StartSubscription(sub, p)
	sub.ProviderType = "credit"
	sub.PeriodStart, sub.PeriodEnd = now.AddDate(0, -1, -1), now.AddDate(0, 0, -1)
	if err := sub.Create(); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.CreatePaidFirstInvoice(db, sub, "credit", "credit_burn"); err != nil {
		t.Fatal(err)
	}
	if err := sub.Update(); err != nil {
		t.Fatal(err)
	}
	if err := refuseUnrealigned(ctx, []*organization.Organization{eco}, ""); err != nil {
		t.Fatalf("before the cycle: %v", err)
	}

	only(t, cycleAt(t, ctx, eco, now, false), engine.Comped)
	got := reloadSub(t, db, sub.Id())
	t.Logf("comped row now %s..%s, CurrentInvoiceId=%s", got.PeriodStart.Format(time.RFC3339), got.PeriodEnd.Format(time.RFC3339), got.CurrentInvoiceId)
	if err := refuseUnrealigned(ctx, []*organization.Organization{eco}, ""); err != nil {
		r := Realign(ctx, []*organization.Organization{eco}, true)
		t.Fatalf("after one live cycle the next is refused: %v; realign would move it to %+v", err, r.Results)
	}
}

// oneDarkSquare answers the first charge 502 without taking it (the request
// never reached Square), then charges every later request.
type oneDarkSquare struct {
	*MockSquareProcessor
	calls    int
	captured int64
}

func (d *oneDarkSquare) Charge(ctx context.Context, req processor.PaymentRequest) (*processor.PaymentResult, error) {
	d.calls++
	if d.calls == 1 {
		err := squarecore.NewAPIError(502, nil, errors.New("bad gateway"))
		return &processor.PaymentResult{Success: false, Error: err, ErrorMessage: err.Error()}, err
	}
	d.captured += int64(req.Amount)
	return &processor.PaymentResult{Success: true, ProcessorRef: "sqpay_" + req.IdempotencyKey, TransactionID: "sqpay_" + req.IdempotencyKey, Status: "COMPLETED"}, nil
}

// A renewal charge that never reached Square (502 from the edge) is unknown.
// The customer cancels at once. The next cycle repeats the attempt under its
// key: Square has never seen the key, so it takes a new payment, after the
// cancel, for a period the customer will not be served.
func TestRed4_CanceledCustomerIsChargedByTheRepeat(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("red4-resolve")
	d := &oneDarkSquare{MockSquareProcessor: newMockSquare(nil, "", nil)}
	old := processorsForOrg
	processorsForOrg = func(*organization.Organization) *processor.Registry {
		reg := processor.NewRegistry(processor.DefaultConfig())
		reg.Register(d)
		return reg
	}
	t.Cleanup(func() { processorsForOrg = old })
	db := datastore.New(org.Namespaced(ctx))
	now := time.Now()
	sub := cardSubThrough(t, db, "red4-resolve", now.Add(-time.Hour))
	only(t, cycleAt(t, ctx, org, now, false), engine.Skipped)
	if _, err := cancelSubscription(ctx, org, sub.Id(), false); err != nil {
		t.Fatal(err)
	}
	r := cycleAt(t, ctx, org, now.Add(time.Hour), false)
	for _, res := range acted(r) {
		t.Logf("after cancel: %s %s %d %s", res.SubscriptionId, res.Action, res.AmountCents, res.Reason)
	}
	for _, inv := range invoicesForSub(t, db, sub.Id()) {
		t.Logf("invoice %s %s paid=%d", inv.Id(), inv.Status, inv.AmountPaid)
		if inv.Status == billinginvoice.Paid && d.captured > 0 {
			t.Fatalf("the customer canceled before any money moved; the cycle then took %d cents and kept them (refund by hand)", d.captured)
		}
	}
}

// A team that lapsed is bought again with the same member. hasMemberSub still
// finds the member's old seat row (active, following the lapsed team), so no
// seat is made under the new team; the next cycle ends the old seat with its
// replaced team, and the member is left with none.
func TestRed4_RebuyingALapsedTeamDropsItsMembers(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("red4-team")
	withFakeSquare(t, squareMock("", "", "sqpay_team4"))
	db := datastore.New(org.Namespaced(ctx))
	p, err := resolveSubscriptionPlan(db, "team")
	if err != nil {
		t.Fatal(err)
	}
	req := func() *createSubscriptionRequest {
		return &createSubscriptionRequest{UserId: "red4-team", PlanId: "team", Quantity: minSeats("team"), Members: []string{"red4-team/bob"}}
	}
	old, err := createSubscription(db, p, req())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	old.ProviderType = "square"
	old.PeriodStart, old.PeriodEnd = now.AddDate(0, -2, 0), now.AddDate(0, -1, 0)
	if err := old.Update(); err != nil {
		t.Fatal(err)
	}
	seats, _ := orgSubscriptions(db, "red4-team/bob")
	for _, s := range seats {
		s2 := reloadSub(t, db, s.Id())
		s2.PeriodStart, s2.PeriodEnd = old.PeriodStart, old.PeriodEnd // as followParent leaves it
		if err := s2.Update(); err != nil {
			t.Fatal(err)
		}
	}

	fresh, err := createSubscription(db, p, req())
	if err != nil {
		t.Fatal(err)
	}
	fresh.ProviderType = "square"
	if err := fresh.Update(); err != nil {
		t.Fatal(err)
	}
	retireLapsed(ctx, org, db, fresh, nil)
	t.Logf("old team %s, new team %s", reloadSub(t, db, old.Id()).Status, reloadSub(t, db, fresh.Id()).Status)

	cycleAt(t, ctx, org, now, false)
	live := 0
	seats, _ = orgSubscriptions(db, "red4-team/bob")
	for _, s := range seats {
		t.Logf("bob seat %s parent=%v %s..%s", s.Status, s.Metadata["bundleParent"], s.PeriodStart.Format(time.RFC3339), s.PeriodEnd.Format(time.RFC3339))
		if s.Status != subscription.Canceled {
			live++
		}
	}
	if live == 0 || tierOf(t, ctx, org, "red4-team/bob") == "free" {
		t.Fatalf("bob was named a member of the new team and holds no live seat (tier %s)", tierOf(t, ctx, org, "red4-team/bob"))
	}
}

// Before the first live cycle after deploy, a paying team's member whose seat
// row is older than a month plus the grace is lapsed: nothing has yet moved the
// seat onto its team's period.
func TestRed4_SeatMemberIsFreeUntilTheFirstLiveCycle(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("red4-seatdeploy")
	db := datastore.New(org.Namespaced(ctx))
	p, _ := resolveSubscriptionPlan(db, "team")
	owner, err := createSubscription(db, p, &createSubscriptionRequest{UserId: "red4-sd", PlanId: "team", Quantity: minSeats("team"), Members: []string{"red4-sd/bob"}})
	if err != nil {
		t.Fatal(err)
	}
	owner.ProviderType = "square"
	if err := owner.Update(); err != nil {
		t.Fatal(err)
	}
	seats, _ := orgSubscriptions(db, "red4-sd/bob")
	s := reloadSub(t, db, seats[0].Id())
	s.PeriodStart, s.PeriodEnd = time.Now().AddDate(0, -3, 0), time.Now().AddDate(0, -2, 0) // made three months ago
	if err := s.Update(); err != nil {
		t.Fatal(err)
	}
	t.Logf("owner %s through %s; bob tier %s, owner tier %s", owner.Status, owner.PeriodEnd.Format(time.RFC3339),
		tierOf(t, ctx, org, "red4-sd/bob"), tierOf(t, ctx, org, "red4-sd"))
	if tierOf(t, ctx, org, "red4-sd/bob") == "free" {
		t.Fatalf("a member of a team paid through %s is Free until a live cycle moves the seat", owner.PeriodEnd.Format(time.RFC3339))
	}
}

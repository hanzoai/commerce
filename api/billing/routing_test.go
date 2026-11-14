package billing

import (
	"bytes"
	"context"
	"github.com/hanzoai/commerce/util/test/ae"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/auth"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/billingevent"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/paymentmethod"
	"github.com/hanzoai/commerce/models/routingpolicy"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/payment/routing"
	"github.com/hanzoai/commerce/util/bit"
	"github.com/hanzoai/commerce/util/permission"
)

// heldStore holds the test datastore open for the life of the test: it is closed when
// its last holder lets go, and a test that holds none is borrowing another's.
func heldStore(t *testing.T) context.Context {
	t.Helper()
	ctx := ae.NewContext()
	t.Cleanup(ctx.Close)
	return ctx
}

func routingEngine(ctx context.Context, claims *auth.IAMClaims) *zip.App {
	eng := zip.New(zip.Config{DisableStartupMessage: true})
	eng.Use(zip.H(func(c *zip.Ctx) error {
		c.SetContext(ctx)
		c.Locals("iam_authenticated", true)
		c.Locals("permissions", bit.Field(permission.Admin|permission.Live))
		c.Locals("iam_claims", claims)
		return c.Next()
	}))
	Route(eng.Group("/v1"))
	return eng
}

func putRouting(eng *zip.App, body string) *http.Response {
	req := httptest.NewRequest(http.MethodPut, "/v1/billing/routing", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := eng.Test(req, patient)
	if err != nil {
		panic(err)
	}
	return resp
}

const sevenTenths = `{"squareWeight":70,"squareOn":true,"stripeOn":true,"localOn":true}`

func TestAnOrgAdminCannotChangeRouting(t *testing.T) {
	ctx := heldStore(t)
	resp := putRouting(routingEngine(ctx, &auth.IAMClaims{Owner: "acme", IsAdmin: true}), sevenTenths)
	if resp.StatusCode != 403 {
		t.Fatalf("an org admin changing the routing got %d, want 403", resp.StatusCode)
	}
	if p, err := ReadRouting(ctx, "acme"); err != nil || p.SquareWeight != 50 {
		t.Fatalf("the refused write changed the policy: %+v %v", p, err)
	}
}

func TestASuperAdminChangesRoutingAndItIsAudited(t *testing.T) {
	ctx := heldStore(t)
	admin := &auth.IAMClaims{Owner: "admin", Email: "z@hanzo.ai", IsAdmin: true, Sudo: true}
	resp := putRouting(routingEngine(ctx, admin), sevenTenths)
	if resp.StatusCode != 200 {
		t.Fatalf("a SuperAdmin changing the routing got %d", resp.StatusCode)
	}
	p, err := ReadRouting(ctx, "hanzo")
	if err != nil || p.SquareWeight != 70 {
		t.Fatalf("the platform policy did not take: %+v %v", p, err)
	}
	// An org's own row overrides the platform's.
	if _, f := WriteRouting(ctx, "acme", routing.Policy{SquareWeight: 0, SquareOn: true, StripeOn: true}, "z@hanzo.ai"); f != nil {
		t.Fatal(f.Err)
	}
	if p, _ := ReadRouting(ctx, "acme"); p.SquareWeight != 0 {
		t.Fatalf("the org's row lost to the platform's: %+v", p)
	}

	rows := make([]*billingevent.BillingEvent, 0)
	if _, err := billingevent.Query(webhookGuardDB(context.Background())).Filter("Type=", "routing.changed").GetAll(&rows); err != nil || len(rows) < 2 {
		t.Fatalf("want a routing.changed record per change, got %d (%v)", len(rows), err)
	}
	var found bool
	for _, r := range rows {
		if r.Data["by"] == "z@hanzo.ai" && r.PreviousData != nil {
			found = true
		}
	}
	if !found {
		t.Fatal("the audit record names neither who made the change nor the policy it replaced")
	}
}

func TestARoutingNobodyCanServeIsRefused(t *testing.T) {
	ctx := heldStore(t)
	_, f := WriteRouting(ctx, "", routing.Policy{SquareWeight: 50}, "z")
	if f == nil || f.Status != 400 {
		t.Fatalf("both processors off must be refused, got %+v", f)
	}
}

// rail is a processor that is there or not, and takes any card. The interface is
// embedded nil: routing never charges, and a call that did would panic loudly.
type rail struct {
	processor.PaymentProcessor
	kind processor.ProcessorType
	up   bool
}

func (r rail) Type() processor.ProcessorType      { return r.kind }
func (r rail) IsAvailable(context.Context) bool   { return r.up }
func (rail) SupportedCurrencies() []currency.Type { return []currency.Type{currency.USD, currency.EUR} }

// Eligible is Square's answer for a US/USD merchant location: only dollars, and only while it is up.
func (r rail) Eligible(_ context.Context, cur currency.Type) (string, bool) {
	return "US", r.kind == processor.Square && r.up && cur == currency.USD
}
func (rail) Reaches(string, string) bool              { return true }
func newRail(t processor.ProcessorType, up bool) rail { return rail{kind: t, up: up} }

func useRails(t *testing.T, square, stripe bool) {
	t.Helper()
	old := processorsForOrg
	processorsForOrg = func(*organization.Organization) *processor.Registry {
		reg := processor.NewRegistry(processor.DefaultConfig())
		reg.SetConfig(&processor.RegistryConfig{ProcessorPriority: []processor.ProcessorType{processor.Square, processor.Stripe}, DisabledProcessors: map[processor.ProcessorType]bool{}})
		reg.Register(newRail(processor.Square, square))
		reg.Register(newRail(processor.Stripe, stripe))
		return reg
	}
	t.Cleanup(func() { processorsForOrg = old })
}

func routingOrg(ctx context.Context, t *testing.T) *organization.Organization {
	t.Helper()
	org := organization.New(datastore.New(ctx))
	org.Name = "routing-" + t.Name()
	if err := org.Create(); err != nil {
		t.Fatal(err)
	}
	return org
}

func TestAProcessorThatIsDownHandsNewBuyersToTheOtherNeverASavedCard(t *testing.T) {
	ctx := heldStore(t)
	org := routingOrg(ctx, t)
	if _, f := WriteRouting(ctx, org.Name, routing.Policy{SquareWeight: 100, SquareOn: true, StripeOn: true, LocalOn: true}, "z"); f != nil {
		t.Fatal(f.Err)
	}

	useRails(t, true, true)
	if d, f := Place(ctx, org, RouteIn{Subject: "new-buyer", Country: "US"}); f != nil || d.Processor != processor.Square || d.Reason != "split" {
		t.Fatalf("up: %+v %v", d, f)
	}

	useRails(t, false, true)
	d, f := Place(ctx, org, RouteIn{Subject: "new-buyer", Country: "US"})
	if f != nil || d.Processor != processor.Stripe || d.Reason != "merchant" {
		t.Fatalf("a new buyer should go to Stripe while Square is down: %+v %v", d, f)
	}

	// A buyer whose card is saved at Square is not handed to Stripe.
	db := datastore.New(org.Namespaced(ctx))
	pm := paymentmethod.New(db)
	pm.CustomerId, pm.Type, pm.ProviderType, pm.IsDefault = "holder", "card", "square", true
	if err := pm.Create(); err != nil {
		t.Fatal(err)
	}
	if d, f := Place(ctx, org, RouteIn{Subject: "holder", Country: "DE"}); f == nil {
		t.Fatalf("a Square holder must not be moved to Stripe while Square is down: %+v", d)
	}
	useRails(t, true, true)
	if d, f := Place(ctx, org, RouteIn{Subject: "holder", Country: "DE"}); f != nil || d.Processor != processor.Square || d.Reason != "saved" {
		t.Fatalf("a saved Square card stays on Square: %+v %v", d, f)
	}
}

func TestEveryAnswerIsRecordedForTheAdminReadout(t *testing.T) {
	ctx := heldStore(t)
	org := routingOrg(ctx, t)
	useRails(t, true, true)
	for i := 0; i < 3; i++ {
		if _, f := Place(ctx, org, RouteIn{Subject: "reader", Country: "BR"}); f != nil {
			t.Fatal(f.Err)
		}
	}
	v, f := ReadRoutingView(ctx, org, org.Name)
	if f != nil {
		t.Fatal(f.Err)
	}
	var stripe int
	for _, c := range v.Counts {
		if c.Processor == "stripe" {
			stripe = c.Count
		}
	}
	if stripe < 3 || len(v.Recent) == 0 || v.Recent[0].Country != "BR" {
		t.Fatalf("the readout lost the decisions: %+v", v)
	}
}

func TestPayPalIsOfferedBesideTheCardWhereItOperatesAndTheDeploymentAllowsIt(t *testing.T) {
	ctx := heldStore(t)
	org := routingOrg(ctx, t)
	t.Setenv(processor.ChargeProcessors, "stripe,square,paypal")
	old := processorsForOrg
	processorsForOrg = func(*organization.Organization) *processor.Registry {
		reg := processor.NewRegistry(processor.DefaultConfig())
		for _, k := range []processor.ProcessorType{processor.Square, processor.Stripe, processor.PayPal} {
			reg.Register(newRail(k, true))
		}
		return reg
	}
	t.Cleanup(func() { processorsForOrg = old })

	d, f := Place(ctx, org, RouteIn{Subject: "p", Country: "BR"})
	if f != nil || len(d.Alternatives) == 0 || d.Alternatives[0] != "paypal" {
		t.Fatalf("a Brazilian buyer should be offered PayPal: %+v %v", d, f)
	}
	if d, _ = Place(ctx, org, RouteIn{Subject: "p", Country: "PK"}); len(d.Alternatives) != 0 {
		t.Fatalf("PayPal does not operate in PK: %+v", d.Alternatives)
	}
	pol := routing.Default()
	pol.PayPalOn = false
	if _, f := WriteRouting(ctx, org.Name, pol, "z"); f != nil {
		t.Fatal(f.Err)
	}
	if d, _ = Place(ctx, org, RouteIn{Subject: "p", Country: "BR"}); len(d.Alternatives) != 0 {
		t.Fatalf("the admin switched PayPal off: %+v", d.Alternatives)
	}
	t.Setenv(processor.ChargeProcessors, "stripe,square")
	pol.PayPalOn = true
	_, _ = WriteRouting(ctx, org.Name, pol, "z")
	if d, _ = Place(ctx, org, RouteIn{Subject: "p", Country: "BR"}); len(d.Alternatives) != 0 {
		t.Fatalf("the deployment does not allow PayPal: %+v", d.Alternatives)
	}
}

func TestTheBackfillListsBillingKeyNamesAndWritesOnlyWhenAsked(t *testing.T) {
	ctx := heldStore(t)
	org := routingOrg(ctx, t)
	db := datastore.New(org.Namespaced(ctx))
	pm := paymentmethod.New(db)
	pm.CustomerId, pm.Type, pm.ProviderType = "github", "card", "stripe"
	pm.Metadata = map[string]interface{}{"customerId": "cus_x"}
	if err := pm.Create(); err != nil {
		t.Fatal(err)
	}
	d := &profileDouble{name: "github"}
	old := processorsForOrg
	processorsForOrg = func(*organization.Organization) *processor.Registry {
		reg := processor.NewRegistry(processor.DefaultConfig())
		reg.Register(d)
		return reg
	}
	t.Cleanup(func() { processorsForOrg = old })
	who := func(context.Context, string) (string, string, bool) { return "Ada Lovelace", "ada@example.com", true }

	rows, err := BackfillPayers(ctx, org, who, false)
	if err != nil || len(rows) != 1 || rows[0].ProposedName != "Ada Lovelace" || rows[0].CurrentName != "github" || rows[0].Written {
		t.Fatalf("dry run = %+v %v", rows, err)
	}
	if d.wrote != 0 {
		t.Fatal("a dry run wrote")
	}
	rows, _ = BackfillPayers(ctx, org, who, true)
	if len(rows) != 1 || !rows[0].Written || d.wrote != 1 {
		t.Fatalf("write = %+v wrote %d", rows, d.wrote)
	}
	if !isBillingKey("Github", "github", "github") || isBillingKey("Ada Lovelace", "github", "github") {
		t.Fatal("isBillingKey")
	}
}

// profileDouble is a Stripe that holds one customer and counts profile writes.
type profileDouble struct {
	processor.PaymentProcessor
	name  string
	wrote int
}

func (*profileDouble) Type() processor.ProcessorType    { return processor.Stripe }
func (*profileDouble) IsAvailable(context.Context) bool { return true }
func (*profileDouble) SupportedCurrencies() []currency.Type {
	return nil
}
func (d *profileDouble) GetCustomer(context.Context, string) (map[string]interface{}, error) {
	return map[string]interface{}{"name": d.name, "email": ""}, nil
}
func (d *profileDouble) CreateCustomerFor(context.Context, processor.Payer, string, map[string]interface{}) (string, error) {
	return "cus_x", nil
}
func (d *profileDouble) ProfileCustomer(context.Context, string, processor.Payer) error {
	d.wrote++
	return nil
}

func TestACardIsSquaresOnlyWhereTheMerchantLocationCanTakeIt(t *testing.T) {
	ctx := heldStore(t)
	org := routingOrg(ctx, t)
	if _, f := WriteRouting(ctx, org.Name, routing.Policy{SquareWeight: 100, SquareOn: true, StripeOn: true, LocalOn: true}, "z"); f != nil {
		t.Fatal(f.Err)
	}
	useRails(t, true, true)

	// A dollar checkout from a German buyer: eligible for the split, and at 100% it is Square's.
	d, f := Place(ctx, org, RouteIn{Subject: "de-buyer", Country: "DE", Currency: "usd"})
	if f != nil || d.Processor != processor.Square || d.Reason != "split" {
		t.Fatalf("a USD checkout from DE must be eligible for Square: %+v %v", d, f)
	}
	// A euro checkout with only a US/USD Square location: Stripe's, decided before any payment.
	d, f = Place(ctx, org, RouteIn{Subject: "de-buyer", Country: "FR", Currency: "eur"})
	if f != nil || d.Processor != processor.Stripe || d.Reason != "merchant" {
		t.Fatalf("a EUR card checkout with a USD-only location must be Stripe's: %+v %v", d, f)
	}
}

func TestAStoredAssignmentSurvivesAnIneligibleCheckoutAndAppliesAgain(t *testing.T) {
	ctx := heldStore(t)
	org := routingOrg(ctx, t)
	if _, f := WriteRouting(ctx, org.Name, routing.Policy{SquareWeight: 100, SquareOn: true, StripeOn: true}, "z"); f != nil {
		t.Fatal(f.Err)
	}
	useRails(t, true, true)
	first, f := Place(ctx, org, RouteIn{Subject: "sticky", Country: "DE", Currency: "usd"})
	if f != nil || first.Processor != processor.Square || first.Reason != "split" {
		t.Fatalf("first placement: %+v %v", first, f)
	}
	// The weight moves to Stripe; the placed customer stays on Square.
	if _, f := WriteRouting(ctx, org.Name, routing.Policy{SquareWeight: 0, SquareOn: true, StripeOn: true}, "z"); f != nil {
		t.Fatal(f.Err)
	}
	if d, _ := Place(ctx, org, RouteIn{Subject: "sticky", Country: "DE", Currency: "usd"}); d.Processor != processor.Square || d.Reason != "assigned" {
		t.Fatalf("a placed customer was re-randomized by a new weight: %+v", d)
	}
	// A euro checkout: Square cannot take it, Stripe does, and the assignment is untouched.
	if d, _ := Place(ctx, org, RouteIn{Subject: "sticky", Country: "DE", Currency: "eur"}); d.Processor != processor.Stripe {
		t.Fatalf("an ineligible checkout was not Stripe's: %+v", d)
	}
	a, found, _ := routingpolicy.Assigned(ctx, org.Name, "sticky")
	if !found || a.Processor != "square" {
		t.Fatalf("the stored assignment changed: %+v", a)
	}
	if d, _ := Place(ctx, org, RouteIn{Subject: "sticky", Country: "FR", Currency: "usd"}); d.Processor != processor.Square {
		t.Fatalf("the assignment did not apply again at the next eligible checkout: %+v", d)
	}
}

func TestNoOtherProcessorIsOfferedWhileASquareChargeIsUnresolved(t *testing.T) {
	ctx := heldStore(t)
	org := routingOrg(ctx, t)
	useRails(t, true, true)
	if err := routingpolicy.Begin(ctx, org.Name, "midway", "square", "attempt-1"); err != nil {
		t.Fatal(err)
	}
	// Square goes down mid-attempt: the payer is held, not handed to Stripe to pay twice.
	useRails(t, false, true)
	if d, f := Place(ctx, org, RouteIn{Subject: "midway", Country: "DE"}); f == nil || f.Status != 409 {
		t.Fatalf("a payer with an unresolved Square charge was offered %+v (%v)", d, f)
	}
	// Known now, whichever way it went: they may check out again, and Stripe takes it.
	if err := routingpolicy.End(ctx, org.Name, "midway", "attempt-1"); err != nil {
		t.Fatal(err)
	}
	if d, f := Place(ctx, org, RouteIn{Subject: "midway", Country: "DE"}); f != nil || d.Processor != processor.Stripe {
		t.Fatalf("after the attempt ended: %+v %v", d, f)
	}
}

package billing

import (
	"bytes"
	"context"
	"errors"
	"github.com/hanzoai/commerce/payment/fx"
	"github.com/hanzoai/commerce/util/test/ae"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
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
	stripeprovider "github.com/hanzoai/commerce/payment/providers/stripe"
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

const sevenTenths = `{"processors":[{"processor":"square","on":true,"weight":70},{"processor":"stripe","on":true,"weight":30}],"localOn":true}`

func TestAnOrgAdminCannotChangeRouting(t *testing.T) {
	ctx := heldStore(t)
	resp := putRouting(routingEngine(ctx, &auth.IAMClaims{Owner: "acme", IsAdmin: true}), sevenTenths)
	if resp.StatusCode != 403 {
		t.Fatalf("an org admin changing the routing got %d, want 403", resp.StatusCode)
	}
	if p, err := ReadRouting(ctx, "acme"); err != nil || squareWeightOf(p) != 50 {
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
	if err != nil || squareWeightOf(p) != 70 {
		t.Fatalf("the platform policy did not take: %+v %v", p, err)
	}
	// An org's own row overrides the platform's.
	if _, f := WriteRouting(ctx, "acme", withWeight(0), "z@hanzo.ai", ""); f != nil {
		t.Fatal(f.Err)
	}
	if p, _ := ReadRouting(ctx, "acme"); squareWeightOf(p) != 0 {
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
	_, f := WriteRouting(ctx, "", routing.Policy{Processors: []routing.ProcessorRule{{Processor: "square", Weight: 50}}}, "z", "")
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

func (r rail) Type() processor.ProcessorType    { return r.kind }
func (r rail) IsAvailable(context.Context) bool { return r.up }
func (r rail) SupportedCurrencies() []currency.Type {
	if r.kind == processor.Square {
		return []currency.Type{currency.USD} // a US/USD Square location
	}
	return stripeprovider.Presentment
}

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
	if _, f := WriteRouting(ctx, org.Name, func() routing.Policy { q := withWeight(100); return q }(), "z", ""); f != nil {
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
	setOn(&pol, "paypal", false)
	if _, f := WriteRouting(ctx, org.Name, pol, "z", ""); f != nil {
		t.Fatal(f.Err)
	}
	if d, _ = Place(ctx, org, RouteIn{Subject: "p", Country: "BR"}); len(d.Alternatives) != 0 {
		t.Fatalf("the admin switched PayPal off: %+v", d.Alternatives)
	}
	t.Setenv(processor.ChargeProcessors, "stripe,square")
	setOn(&pol, "paypal", true)
	_, _ = WriteRouting(ctx, org.Name, pol, "z", "")
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
	if _, f := WriteRouting(ctx, org.Name, func() routing.Policy { q := withWeight(100); return q }(), "z", ""); f != nil {
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
	if _, f := WriteRouting(ctx, org.Name, withWeight(100), "z", ""); f != nil {
		t.Fatal(f.Err)
	}
	useRails(t, true, true)
	first, f := Place(ctx, org, RouteIn{Subject: "sticky", Country: "DE", Currency: "usd"})
	if f != nil || first.Processor != processor.Square || first.Reason != "split" {
		t.Fatalf("first placement: %+v %v", first, f)
	}
	// The weight moves to Stripe; the placed customer stays on Square.
	if _, f := WriteRouting(ctx, org.Name, withWeight(0), "z", ""); f != nil {
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
	if err := routingpolicy.Begin(ctx, org.Name, "midway", "square", "attempt-1", 1); err != nil {
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

// squareWeightOf is Square's weight in a policy.
func squareWeightOf(p routing.Policy) int {
	for _, r := range p.Processors {
		if r.Processor == "square" {
			return r.Weight
		}
	}
	return -1
}

// setWeights sets the weights of Square and Stripe.
func setWeights(p *routing.Policy, square, stripe int) {
	for i := range p.Processors {
		switch p.Processors[i].Processor {
		case "square":
			p.Processors[i].Weight = square
		case "stripe":
			p.Processors[i].Weight = stripe
		}
	}
}

// setOn switches a processor in or out.
func setOn(p *routing.Policy, name string, on bool) {
	for i := range p.Processors {
		if p.Processors[i].Processor == name {
			p.Processors[i].On = on
		}
	}
}

// withWeight is the default policy with only the weight changed.
func withWeight(w int) routing.Policy {
	p := routing.Default()
	setWeights(&p, w, 100-w)
	return p
}

func TestAnIneligibleOverrideIsRefusedAtSaveAndChangesNothing(t *testing.T) {
	ctx := heldStore(t)
	useRails(t, true, true) // a US/USD Square location, Stripe in usd, eur, gbp
	scope := "rules-" + t.Name()
	before := withWeight(70)
	if _, f := WriteRouting(ctx, scope, before, "z", "base"); f != nil {
		t.Fatal(f.Err)
	}
	for name, mutate := range map[string]func(*routing.Policy){
		"euro to a USD-only Square": func(p *routing.Policy) {
			p.CurrencyRules = []routing.CurrencyRule{{Currency: "eur", Processor: "square"}}
		},
		"unknown brand": func(p *routing.Policy) { p.BrandRules = []routing.BrandRule{{Brand: "amexx", Processor: "stripe"}} },
		"not a card processor": func(p *routing.Policy) {
			p.CurrencyRules = []routing.CurrencyRule{{Currency: "usd", Processor: "paypal"}}
		},
		"unknown method": func(p *routing.Policy) { p.LocalMethods = []routing.MethodRule{{Method: "bitcoin", On: false}} },
		"bad issuer country": func(p *routing.Policy) {
			p.IssuerCountryRules = []routing.IssuerRule{{Country: "USA", Processor: "stripe"}}
		},
		"fallback to an unknown name": func(p *routing.Policy) { p.Fallback = []string{"adyen"} },
		"stripe for a currency it lacks": func(p *routing.Policy) {
			p.CurrencyRules = []routing.CurrencyRule{{Currency: "kwd", Processor: "stripe"}}
		},
		"a rule named twice": func(p *routing.Policy) {
			p.CountryRules = []routing.CountryRule{{Country: "DE", Processor: "stripe"}, {Country: "DE", Processor: "square"}}
		},
	} {
		p := withWeight(70)
		mutate(&p)
		if _, f := WriteRouting(ctx, scope, p, "z", ""); f == nil || f.Status != 400 {
			t.Fatalf("%s: want a 400 refusal, got %v", name, f)
		}
	}
	if got, _ := ReadRouting(ctx, scope); squareWeightOf(got) != squareWeightOf(before) || len(got.CurrencyRules) != 0 {
		t.Fatalf("a refused save changed the policy: %+v", got)
	}
	if vs, _ := ListRoutingVersions(ctx, scope); len(vs) != 1 {
		t.Fatalf("a refused save made a version: %d", len(vs))
	}
	refusals := make([]*billingevent.BillingEvent, 0)
	if _, err := billingevent.Query(webhookGuardDB(ctx)).Filter("Type=", "routing.refused").GetAll(&refusals); err != nil || len(refusals) < 8 {
		t.Fatalf("the refusals were not recorded: %d %v", len(refusals), err)
	}
}

func TestEachRuleChangesRoutingAsConfigured(t *testing.T) {
	ctx := heldStore(t)
	useRails(t, true, true)
	org := routingOrg(ctx, t)
	save := func(mutate func(*routing.Policy)) {
		t.Helper()
		p := withWeight(100)
		mutate(&p)
		if _, f := WriteRouting(ctx, org.Name, p, "z", ""); f != nil {
			t.Fatalf("save: %v", f.Err)
		}
	}
	who := func(sub string, in RouteIn) *routing.Decision {
		t.Helper()
		in.Subject = sub
		if in.Currency == "" {
			in.Currency = "usd"
		}
		d, f := Place(ctx, org, in)
		if f != nil {
			t.Fatalf("place: %v", f.Err)
		}
		return d
	}
	// The weight alone: Square.
	save(func(*routing.Policy) {})
	if d := who("a", RouteIn{Country: "FR"}); d.Processor != processor.Square || d.Rule != "split" {
		t.Fatalf("weight: %+v", d)
	}
	// A currency rule.
	save(func(p *routing.Policy) {
		p.CurrencyRules = []routing.CurrencyRule{{Currency: "usd", Processor: "stripe"}}
	})
	if d := who("b", RouteIn{Country: "FR"}); d.Processor != processor.Stripe || d.Rule != "currency" {
		t.Fatalf("currency rule: %+v", d)
	}
	// A buyer-country rule.
	save(func(p *routing.Policy) { p.CountryRules = []routing.CountryRule{{Country: "BR", Processor: "stripe"}} })
	if d := who("c", RouteIn{Country: "BR"}); d.Processor != processor.Stripe || d.Rule != "country" {
		t.Fatalf("country rule: %+v", d)
	}
	if d := who("c2", RouteIn{Country: "FR"}); d.Processor != processor.Square {
		t.Fatalf("a country rule moved another country: %+v", d)
	}
	// The buyer's explicit method.
	save(func(*routing.Policy) {})
	if d := who("d", RouteIn{Country: "US", Method: "cash_app"}); d.Processor != processor.Square || d.Rule != "method" {
		t.Fatalf("cash app: %+v", d)
	}
	if d := who("e", RouteIn{Country: "BR", Method: "pix"}); d.Processor != processor.Stripe || d.Rule != "method" {
		t.Fatalf("pix: %+v", d)
	}
	// Card rules apply where the metadata is known: a preview names it, a live new card has none.
	save(func(p *routing.Policy) {
		p.IssuerCountryRules = []routing.IssuerRule{{Country: "GB", Processor: "stripe"}}
		p.BrandRules = []routing.BrandRule{{Brand: "amex", Processor: "stripe"}}
	})
	if d, _ := PreviewRouting(ctx, org, PreviewIn{Country: "US", Currency: "usd", IssuerCountry: "gb"}); d.Processor != processor.Stripe || d.Rule != "issuer" {
		t.Fatalf("issuer: %+v", d)
	}
	if d, _ := PreviewRouting(ctx, org, PreviewIn{Country: "US", Currency: "usd", Brand: "AMEX"}); d.Processor != processor.Stripe || d.Rule != "brand" {
		t.Fatalf("brand: %+v", d)
	}
	if d := who("f", RouteIn{Country: "US"}); d.Processor != processor.Square {
		t.Fatalf("a new card with no metadata was routed by a card rule: %+v", d)
	}
	// A rule to a processor that cannot take the charge now is passed over, and says so.
	save(func(p *routing.Policy) {
		p.CurrencyRules = []routing.CurrencyRule{{Currency: "usd", Processor: "square"}}
	})
	useRails(t, false, true) // Square goes down after the save
	d, f := Place(ctx, org, RouteIn{Subject: "g", Country: "FR", Currency: "usd"})
	if f != nil || d.Processor != processor.Stripe {
		t.Fatalf("a rule to a down processor: %+v %v", d, f)
	}
}

func TestTheChainHasSevenStepsInOrderAndSavedCardsIgnoreEveryRule(t *testing.T) {
	p := routing.Default()
	p.CurrencyRules = []routing.CurrencyRule{{Currency: "usd", Processor: "stripe"}}
	p.CountryRules = []routing.CountryRule{{Country: "DE", Processor: "stripe"}}
	p.BrandRules = []routing.BrandRule{{Brand: "visa", Processor: "stripe"}}
	p.IssuerCountryRules = []routing.IssuerRule{{Country: "DE", Processor: "stripe"}}
	setWeights(&p, 100, 0)
	in := routing.Input{Org: "o", Key: "k", Country: "DE", Currency: "usd", Brand: "visa", IssuerCountry: "DE", Capable: []routing.Capability{{Processor: processor.Square, OK: true}}, Saved: processor.Square, Assigned: processor.Stripe}
	d := p.Decide(in)
	if d.Processor != processor.Square || d.Rule != "saved" {
		t.Fatalf("a saved Square card was moved by a rule: %+v", d)
	}
	names := []string{}
	for _, s := range d.Chain {
		names = append(names, s.Step)
	}
	if got := strings.Join(names, ","); got != "saved,eligibility,method,overrides,sticky,weight,fallback" {
		t.Fatalf("the chain is %s", got)
	}
	if !d.Chain[0].Applied || d.Chain[3].Applied {
		t.Fatalf("saved must apply and overrides be skipped: %+v", d.Chain)
	}
	in.Saved = ""
	d = p.Decide(in)
	if d.Rule != "currency" || !d.Chain[3].Applied {
		t.Fatalf("with no saved card the currency rule decides: %+v", d)
	}
}

func TestStickyOffReevaluatesAndResetClearsAPlacementBothAudited(t *testing.T) {
	ctx := heldStore(t)
	useRails(t, true, true)
	org := routingOrg(ctx, t)
	if _, f := WriteRouting(ctx, org.Name, withWeight(100), "z", ""); f != nil {
		t.Fatal(f.Err)
	}
	if d, _ := Place(ctx, org, RouteIn{Subject: "pin", Country: "DE"}); d.Processor != processor.Square {
		t.Fatalf("first placement: %+v", d)
	}
	off := withWeight(0)
	off.Sticky = false
	if _, f := WriteRouting(ctx, org.Name, off, "z", ""); f != nil {
		t.Fatal(f.Err)
	}
	if d, _ := Place(ctx, org, RouteIn{Subject: "pin", Country: "DE"}); d.Processor != processor.Stripe {
		t.Fatalf("sticky off must re-evaluate: %+v", d)
	}
	on := withWeight(0) // sticky on, weight now Stripe's; "pin" is still placed on Square
	if _, f := WriteRouting(ctx, org.Name, on, "z", ""); f != nil {
		t.Fatal(f.Err)
	}
	if d, _ := Place(ctx, org, RouteIn{Subject: "pin", Country: "DE"}); d.Processor != processor.Square || d.Rule != "assigned" {
		t.Fatalf("the placement should hold: %+v", d)
	}
	n, f := ResetRouting(ctx, org.Name, "pin", "z@hanzo.ai")
	if f != nil || n != 1 {
		t.Fatalf("reset: %d %v", n, f)
	}
	if d, _ := Place(ctx, org, RouteIn{Subject: "pin", Country: "DE"}); d.Processor != processor.Stripe || d.Rule != "split" {
		t.Fatalf("after the reset the payer is placed afresh: %+v", d)
	}
	rows := make([]*billingevent.BillingEvent, 0)
	if _, err := billingevent.Query(webhookGuardDB(ctx)).Filter("Type=", "routing.reset").GetAll(&rows); err != nil || len(rows) < 1 || rows[0].Data["by"] != "z@hanzo.ai" {
		t.Fatalf("the reset was not audited: %v %v", rows, err)
	}
}

func TestVersionsAreImmutableListedAndRestoredAsNewVersions(t *testing.T) {
	ctx := heldStore(t)
	useRails(t, true, true)
	scope := "ver-" + t.Name()
	for i, w := range []int{20, 60, 90} {
		if _, f := WriteRouting(ctx, scope, withWeight(w), "z@hanzo.ai", "set "+strconv.Itoa(w)); f != nil {
			t.Fatalf("save %d: %v", i, f.Err)
		}
	}
	vs, _ := ListRoutingVersions(ctx, scope)
	if len(vs) != 3 || vs[0].N != 3 || !vs[0].Active || squareWeightOf(vs[0].Policy) != 90 || vs[0].By != "z@hanzo.ai" || vs[0].Note != "set 90" {
		t.Fatalf("history: %+v", vs)
	}
	if len(vs[0].Diff) == 0 || !strings.Contains(strings.Join(vs[0].Diff, ";"), "processors: ") {
		t.Fatalf("the diff against the previous version: %v", vs[0].Diff)
	}
	if err := routingpolicy.PutVersion(ctx, routingpolicy.Version{Scope: scope, N: 1, Policy: "{}"}); err == nil {
		t.Fatal("a stored version was replaced")
	}
	p, f := RestoreRouting(ctx, scope, 1, "z@hanzo.ai", "")
	if f != nil || squareWeightOf(p) != 20 {
		t.Fatalf("restore: %+v %v", p, f)
	}
	vs, _ = ListRoutingVersions(ctx, scope)
	if len(vs) != 4 || vs[0].N != 4 || vs[0].Restored != 1 || !vs[0].Active || squareWeightOf(vs[3].Policy) != 20 || squareWeightOf(vs[1].Policy) != 90 {
		t.Fatalf("a restore must add a version and rewrite nothing: %+v", vs)
	}
	// A version that no longer holds is refused with the reason.
	cur := withWeight(50)
	cur.CurrencyRules = []routing.CurrencyRule{{Currency: "usd", Processor: "square"}}
	if _, f := WriteRouting(ctx, scope, cur, "z", "usd to square"); f != nil {
		t.Fatal(f.Err)
	}
	useRails(t, false, true) // Square's merchant location is gone
	if _, f := RestoreRouting(ctx, scope, 5, "z", ""); f == nil || f.Status != 400 {
		t.Fatalf("a version naming an ineligible processor was restored: %v", f)
	}
	if _, f := RestoreRouting(ctx, scope, 99, "z", ""); f == nil || f.Status != 404 {
		t.Fatalf("a missing version: %v", f)
	}
}

func TestAPolicyChangeReflectsOnTheNextCheckoutWithNoRestart(t *testing.T) {
	ctx := heldStore(t)
	useRails(t, true, true)
	org := routingOrg(ctx, t)
	for i, want := range []processor.ProcessorType{processor.Square, processor.Stripe, processor.Square} {
		p := withWeight(0)
		if want == processor.Square {
			setWeights(&p, 100, 0)
		}
		p.Sticky = false
		if _, f := WriteRouting(ctx, org.Name, p, "z", ""); f != nil {
			t.Fatal(f.Err)
		}
		if d, _ := Place(ctx, org, RouteIn{Subject: "live", Country: "DE"}); d.Processor != want {
			t.Fatalf("change %d did not reach the next checkout: %+v", i, d)
		}
	}
}

func TestTheFallbackOrderIsTheAdminsAndIsUsedOnlyBeforePayment(t *testing.T) {
	ctx := heldStore(t)
	org := routingOrg(ctx, t)
	p := withWeight(100)
	p.Fallback = []string{"stripe"}
	useRails(t, true, true)
	if _, f := WriteRouting(ctx, org.Name, p, "z", ""); f != nil {
		t.Fatal(f.Err)
	}
	useRails(t, false, true) // Square is down before any payment starts
	d, f := Place(ctx, org, RouteIn{Subject: "fb", Country: "DE"})
	if f != nil || d.Processor != processor.Stripe {
		t.Fatalf("pre-payment failover: %+v %v", d, f)
	}
	// Once a Square attempt exists, no failover: the payer waits for its outcome.
	if err := routingpolicy.Begin(ctx, org.Name, "fb2", "square", "att", 1); err != nil {
		t.Fatal(err)
	}
	if _, f := Place(ctx, org, RouteIn{Subject: "fb2", Country: "DE"}); f == nil || f.Status != 409 {
		t.Fatalf("failover after a Square attempt began: %v", f)
	}
}

func TestTheDefaultPolicyIsTodaysBehaviour(t *testing.T) {
	p := routing.Default()
	if squareWeightOf(p) != 50 || !p.On(processor.Square) || !p.On(processor.Stripe) || !p.LocalOn || !p.Sticky || !p.On(processor.PayPal) || !p.On(routing.CryptoRail) {
		t.Fatalf("defaults: %+v", p)
	}
	if len(p.CurrencyRules)+len(p.CountryRules)+len(p.BrandRules)+len(p.IssuerCountryRules)+len(p.Fallback)+len(p.LocalMethods) != 0 || len(p.Off()) != 0 {
		t.Fatalf("the default carries rules: %+v", p)
	}
	// An eligible dollar checkout is the split; an ineligible one is Stripe's.
	if d := p.Decide(routing.Input{Org: "o", Key: "k", Currency: "usd", Capable: []routing.Capability{{Processor: processor.Square, OK: false}}}); d.Processor != processor.Stripe || d.Rule != "merchant" {
		t.Fatalf("%+v", d)
	}
	// Every method is on, and switching one off is visible to the Payment Element.
	p.LocalMethods = []routing.MethodRule{{Method: "pix", On: false}, {Method: "apple_pay", On: false}}
	if got := p.Off(); len(got) != 1 || got[0] != "pix" {
		t.Fatalf("Off() = %v", got)
	}
	if m := p.Methods(processor.Square, "US"); strings.Contains(strings.Join(m, ","), "apple_pay") {
		t.Fatalf("apple_pay is off but mounted: %v", m)
	}
}

func stubRates(t *testing.T, rates map[string]float64) {
	t.Helper()
	old := fx.Rate
	fx.Rate = func(_ context.Context, cur string) (float64, error) {
		if r, ok := rates[cur]; ok {
			return r, nil
		}
		return 0, errors.New("no rate source for " + cur)
	}
	t.Cleanup(func() { fx.Rate = old })
}

func TestTheDefaultPolicyChargesExactlyTheConvertedAmountRoundedUpToTheIncrement(t *testing.T) {
	ctx := heldStore(t)
	useRails(t, true, true)
	stubRates(t, map[string]float64{"eur": 0.9137, "inr": 94.25, "jpy": 150.4, "brl": 5})
	reg := processorsForOrg(&organization.Organization{})

	p := routing.Default()
	// No margin by default, whatever the source: $10 at 0.9137 is 9.137 euro, rounded up to 10.
	q, f := quoteFor(ctx, reg, p, 1000, "DE", "", "", false)
	if f != nil || q.Currency != "eur" || q.Amount != 1000 || q.UsdCents != 1000 || q.Rate != 0.9137 || q.Source != "reference" || q.FXFee != "absorb" {
		t.Fatalf("default: %+v %v", q, f)
	}
	// Go at 94.25: $8 is exactly Rs 754.
	if q, f = quoteFor(ctx, reg, p, 800, "IN", "", "", false); f != nil || q.Amount != 75400 || q.Increment != 100 || q.Decimals != 2 {
		t.Fatalf("IN: %+v %v", q, f)
	}
	// Yen round up to ten: $20 is 3008 yen, charged 3010.
	if q, f = quoteFor(ctx, reg, p, 2000, "JP", "", "", false); f != nil || q.Amount != 3010 || q.Increment != 10 || q.Decimals != 0 {
		t.Fatalf("JP: %+v %v", q, f)
	}
	// The increment is the admin's to set, and rounding can be turned off.
	p.RoundingIncrements = []routing.RoundingIncrement{{Currency: "jpy", Increment: 100}}
	if q, _ = quoteFor(ctx, reg, p, 2000, "JP", "", "", false); q.Amount != 3100 {
		t.Fatalf("a 100 yen increment: %+v", q)
	}
	p.RoundingIncrements, p.NoRounding = nil, true
	if q, _ = quoteFor(ctx, reg, p, 2000, "JP", "", "", false); q.Amount != 3008 {
		t.Fatalf("rounding off: %+v", q)
	}
	for _, bad := range []routing.RoundingIncrement{{Currency: "kwd", Increment: 0.005}, {Currency: "inr", Increment: 0}, {Currency: "isk", Increment: 0.5}} {
		bp := routing.Default()
		bp.RoundingIncrements = []routing.RoundingIncrement{bad}
		if why := bp.Valid(); why == "" {
			t.Errorf("%+v should be refused", bad)
		}
	}
	// The table is a default, never a restriction.
	// The table is a default, never a restriction: BR is brl by default, usd when the
	// admin sets it, and a DE buyer may choose usd or brl whatever the table says.
	p = routing.Default()
	if q, _ = quoteFor(ctx, reg, p, 1000, "BR", "", "", false); q.Currency != "brl" {
		t.Fatalf("BR default: %+v", q)
	}
	p.LocalCurrencies = []routing.LocalCurrencyRule{{Country: "BR", Currency: "usd"}}
	if q, _ = quoteFor(ctx, reg, p, 1000, "BR", "", "", false); q.Currency != "usd" || q.Amount != 1000 {
		t.Fatalf("BR set to usd: %+v", q)
	}
	if q, f = quoteFor(ctx, reg, routing.Default(), 1000, "DE", "usd", "", false); f != nil || q.Currency != "usd" {
		t.Fatalf("a DE buyer choosing usd: %+v %v", q, f)
	}
	if q, f = quoteFor(ctx, reg, routing.Default(), 1000, "DE", "brl", "pix", false); f != nil || q.Currency != "brl" {
		t.Fatalf("a DE buyer choosing pix in brl: %+v %v", q, f)
	}
	// Not viable: refused with a clear 400.
	for _, c := range []struct{ country, cur, method string }{
		{"BR", "usd", "pix"}, {"DE", "", "pix"}, {"DE", "kwd", ""}, {"DE", "xyz", ""},
	} {
		if _, f := quoteFor(ctx, reg, routing.Default(), 1000, c.country, c.cur, c.method, false); f == nil || f.Status != 400 {
			t.Fatalf("%+v should be a 400, got %v", c, f)
		}
	}
}

func TestInvalidFXAndCountryTableEntriesAreRefusedAndTheseFieldsAreVersioned(t *testing.T) {
	ctx := heldStore(t)
	useRails(t, true, true)
	stubRates(t, map[string]float64{"eur": 0.9, "brl": 5, "gbp": 0.8})
	scope := "fx-" + t.Name()
	bad := map[string]func(*routing.Policy){
		"unknown fx cost":   func(p *routing.Policy) { p.FXFee = "split" },
		"unknown fx source": func(p *routing.Policy) { p.FXSource = "oracle" },
		"unknown country": func(p *routing.Policy) {
			p.LocalCurrencies = []routing.LocalCurrencyRule{{Country: "ZZ", Currency: "eur"}}
		},
		"an increment Stripe cannot take": func(p *routing.Policy) {
			p.RoundingIncrements = []routing.RoundingIncrement{{Currency: "kwd", Increment: 0.005}}
		},
		"no rate source": func(p *routing.Policy) {
			p.LocalCurrencies = []routing.LocalCurrencyRule{{Country: "CH", Currency: "chf"}}
		},
		"a country named twice": func(p *routing.Policy) {
			p.LocalCurrencies = []routing.LocalCurrencyRule{{Country: "BR", Currency: "brl"}, {Country: "BR", Currency: "usd"}}
		},
	}
	for name, mut := range bad {
		p := routing.Default()
		mut(&p)
		if _, f := WriteRouting(ctx, scope, p, "z", ""); f == nil || f.Status != 400 {
			t.Fatalf("%s: want a 400, got %v", name, f)
		}
	}
	if vs, _ := ListRoutingVersions(ctx, scope); len(vs) != 0 {
		t.Fatalf("a refused save made a version")
	}
	one := routing.Default()
	one.FXFee = routing.FXAbsorb
	one.LocalCurrencies = []routing.LocalCurrencyRule{{Country: "GB", Currency: "gbp"}, {Country: "BR", Currency: "usd"}}
	if _, f := WriteRouting(ctx, scope, one, "z", "uk in pounds, br in dollars"); f != nil {
		t.Fatal(f.Err)
	}
	two := routing.Default()
	if _, f := WriteRouting(ctx, scope, two, "z", "back to defaults"); f != nil {
		t.Fatal(f.Err)
	}
	vs, _ := ListRoutingVersions(ctx, scope)
	if len(vs) != 2 || !strings.Contains(strings.Join(vs[0].Diff, ";"), "fxFee: \"absorb\" -> \"pass\"") || !strings.Contains(strings.Join(vs[0].Diff, ";"), "localCurrencies") {
		t.Fatalf("the diff misses these fields: %+v", vs)
	}
	if p, f := RestoreRouting(ctx, scope, 1, "z", ""); f != nil || p.FXFee != routing.FXAbsorb || len(p.LocalCurrencies) != 2 {
		t.Fatalf("restore: %+v %v", p, f)
	}
	if got, _ := ReadRouting(ctx, scope); got.FXFee != routing.FXAbsorb || got.LocalCurrency("gb") != "gbp" || got.LocalCurrency("BR") != "usd" || got.LocalCurrency("DE") != "eur" {
		t.Fatalf("the restored policy reads wrong: %+v", got)
	}
}

func TestCardRulesSayTheyDoNotApplyWhenNoCardMetadataIsKnown(t *testing.T) {
	p := routing.Default()
	p.BrandRules = []routing.BrandRule{{Brand: "amex", Processor: "stripe"}}
	p.IssuerCountryRules = []routing.IssuerRule{{Country: "GB", Processor: "stripe"}}
	d := p.Decide(routing.Input{Org: "o", Key: "k", Currency: "usd", Capable: []routing.Capability{{Processor: processor.Square, OK: true}}})
	step := d.Chain[3]
	if step.Step != "overrides" || step.Applied || !strings.Contains(step.Why, "not applicable: card metadata unknown before selection") {
		t.Fatalf("a new card must not be decided by a card rule: %+v", step)
	}
	d = p.Decide(routing.Input{Org: "o", Key: "k", Currency: "usd", Capable: []routing.Capability{{Processor: processor.Square, OK: true}}, Brand: "amex"})
	if !d.Chain[3].Applied || d.Rule != "brand" {
		t.Fatalf("with metadata known the rule applies: %+v", d)
	}
}

func TestAQuoteCountsInTheCurrencysOwnMinorUnitAndKeepsTheCreditExact(t *testing.T) {
	ctx := heldStore(t)
	useRails(t, true, true)
	stubRates(t, map[string]float64{"jpy": 150, "krw": 1400, "inr": 83.5, "huf": 360, "aed": 3.6725})
	reg := processorsForOrg(&organization.Organization{})
	p := routing.Default()
	for _, c := range []struct {
		country, cur string
		amount       int64
		dec          int
	}{
		{"JP", "jpy", 3000, 0}, {"KR", "krw", 28000, 0}, {"IN", "inr", 167000, 2}, {"HU", "huf", 720000, 2}, {"AE", "aed", 7400, 2},
	} {
		q, f := quoteFor(ctx, reg, p, 2000, c.country, "", "", false)
		if f != nil || q.Currency != c.cur || q.Amount != c.amount || q.Decimals != c.dec || q.UsdCents != 2000 {
			t.Errorf("%s: %+v %v, want %d %s", c.country, q, f, c.amount, c.cur)
		}
	}
	// The weakest rate the table can meet still clears Stripe's minimum, and under it is refused.
	if q, f := quoteFor(ctx, reg, p, 49, "JP", "", "", false); q != nil || f == nil || f.Status != 400 {
		t.Errorf("49 cents is under Stripe's minimum: %+v %v", q, f)
	}
	// A currency Stripe cannot present on this account is refused, not sent to fail there.
	if q, f := quoteFor(ctx, reg, p, 2000, "KW", "kwd", "", false); q != nil || f == nil {
		t.Errorf("kwd is not presentable on a US account: %+v", q)
	}
	// The admin's entry wins over the table.
	p.LocalCurrencies = []routing.LocalCurrencyRule{{Country: "JP", Currency: "usd"}, {Country: "US", Currency: "krw"}}
	if q, _ := quoteFor(ctx, reg, p, 2000, "JP", "", "", false); q.Currency != "usd" || q.Amount != 2000 {
		t.Errorf("override JP: %+v", q)
	}
	if q, _ := quoteFor(ctx, reg, p, 2000, "US", "", "", false); q.Currency != "krw" || q.Amount != 28000 {
		t.Errorf("override US: %+v", q)
	}
}

// quotingRail is a Stripe that quotes its own rate, as one with FX Quotes does.
type quotingRail struct {
	rail
	rq    processor.RateQuote
	fail  bool
	locks []string
}

func (r *quotingRail) QuoteRate(_ context.Context, cur, lock string) (*processor.RateQuote, error) {
	r.locks = append(r.locks, lock)
	if r.fail {
		return nil, errors.New("fx quotes are not enabled for this account")
	}
	q := r.rq
	q.Currency, q.Lock = cur, lock
	return &q, nil
}

func useQuotingRail(t *testing.T, r *quotingRail) *processor.Registry {
	t.Helper()
	shownMu.Lock()
	shown = map[string]shownRate{}
	shownMu.Unlock()
	old := processorsForOrg
	processorsForOrg = func(*organization.Organization) *processor.Registry {
		reg := processor.NewRegistry(processor.DefaultConfig())
		reg.SetConfig(&processor.RegistryConfig{ProcessorPriority: []processor.ProcessorType{processor.Stripe}, DisabledProcessors: map[processor.ProcessorType]bool{}})
		reg.Register(r)
		return reg
	}
	t.Cleanup(func() { processorsForOrg = old; shownMu.Lock(); shown = map[string]shownRate{}; shownMu.Unlock() })
	return processorsForOrg(nil)
}

func TestStripesRateIsPassedThroughOrAbsorbedAndBoundToThePayment(t *testing.T) {
	ctx := heldStore(t)
	stubRates(t, map[string]float64{"inr": 90})
	r := &quotingRail{rail: newRail(processor.Stripe, true), rq: processor.RateQuote{ID: "fxq_1", PerDollar: 95.2361, BasePerDollar: 94.2842,
		ExchangeRate: 0.0105002, FeeRate: 0.01, Premium: 0.001, ReferenceRate: 0.0106108, ReferenceProvider: "ecb", ExpiresAt: 1900000000}}
	reg := useQuotingRail(t, r)

	// Pass (the default): $8 at Stripe's effective rate, fee and premium in it: 761.89 -> Rs 762.
	p := routing.Default()
	q, f := quoteFor(ctx, reg, p, 800, "IN", "", "", false)
	if f != nil || q.Amount != 76200 || q.Source != "processor" || q.FXFee != "pass" || q.Rate != 95.2361 || q.QuoteID != "" || q.ExchangeRate != 0.0105002 {
		t.Fatalf("pass: %+v %v", q, f)
	}
	if len(r.locks) != 1 || r.locks[0] != "none" {
		t.Fatalf("a displayed price takes the current rate with no lock: %v", r.locks)
	}
	// A displayed price reuses the cached rate; it does not quote again.
	quoteFor(ctx, reg, p, 800, "IN", "", "", false)
	if len(r.locks) != 1 {
		t.Fatalf("the shown rate was not cached: %v", r.locks)
	}
	// Absorb: base_rate, Hanzo bears the cost: 754.27 -> Rs 755.
	p.FXFee = routing.FXAbsorb
	if q, _ = quoteFor(ctx, reg, p, 800, "IN", "", "", false); q.Amount != 75500 || q.FXFee != "absorb" {
		t.Fatalf("absorb: %+v", q)
	}
	// A payment being opened locks the rate for an hour, a voucher method for a day, and
	// the quote names the lock the intent is bound to.
	p = routing.Default()
	if q, _ = quoteFor(ctx, reg, p, 800, "IN", "", "", true); q.QuoteID != "fxq_1" || q.ReferenceProvider != "ecb" || q.LockExpiresAt != 1900000000 || r.locks[len(r.locks)-1] != "hour" {
		t.Fatalf("bind: %+v %v", q, r.locks)
	}
	r.rq.PerDollar, r.rq.BasePerDollar = 5.1, 5.05
	if q, _ = quoteFor(ctx, reg, p, 1000, "BR", "", "pix", true); q.Amount != 5100 || r.locks[len(r.locks)-1] != "day" {
		t.Fatalf("pix locks a day: %+v %v", q, r.locks)
	}
	// Without FX Quotes the market rate is used, no margin, and the quote says Hanzo
	// bears the conversion cost.
	r.fail = true
	if q, f = quoteFor(ctx, reg, p, 800, "IN", "", "", true); f != nil || q.Source != "reference" || q.FXFee != "absorb" || q.Amount != 72000 || q.QuoteID != "" {
		t.Fatalf("fallback: %+v %v", q, f)
	}
	// The market source can be chosen outright.
	r.fail = false
	p.FXSource = routing.FXReference
	if q, _ = quoteFor(ctx, reg, p, 800, "IN", "", "", false); q.Source != "reference" || q.Amount != 72000 {
		t.Fatalf("market: %+v", q)
	}
}

// A route to Square or to crypto yields a valid quote, and never asks Stripe for a rate.
func TestARouteToSquareOrCryptoQuotesWithoutAskingStripe(t *testing.T) {
	ctx := heldStore(t)
	stripe := &quotingRail{rail: newRail(processor.Stripe, true), rq: processor.RateQuote{ID: "fxq_x", PerDollar: 95, BasePerDollar: 94}}
	old := processorsForOrg
	reg := processor.NewRegistry(processor.DefaultConfig())
	reg.SetConfig(&processor.RegistryConfig{ProcessorPriority: []processor.ProcessorType{processor.Square, processor.Stripe}, DisabledProcessors: map[processor.ProcessorType]bool{}})
	reg.Register(newRail(processor.Square, true))
	reg.Register(stripe)
	processorsForOrg = func(*organization.Organization) *processor.Registry { return reg }
	t.Cleanup(func() { processorsForOrg = old })
	stubRates(t, map[string]float64{"inr": 94.25})

	p := routing.Default()
	for _, via := range []processor.ProcessorType{processor.Square, "crypto"} {
		q, f := quoteVia(ctx, reg, p, via, 2000, "IN", "", "", true)
		if f != nil || q.Currency != "usd" || q.Amount != 2000 || q.UsdCents != 2000 || q.Rate != 1 || q.Source != "none" || q.Decimals != 2 {
			t.Fatalf("%s: %+v %v", via, q, f)
		}
	}
	if q := nativeQuote(2000); q.Source != "none" || q.Rate != 1 || q.Amount != 2000 {
		t.Fatalf("native: %+v", q)
	}
	if len(stripe.locks) != 0 {
		t.Fatalf("Stripe was asked for a rate: %v", stripe.locks)
	}
	// A rail without a quote of its own that can take the currency prices on the
	// reference rate: the same path, another source.
	q, f := quoteVia(ctx, reg, p, processor.Stripe, 800, "IN", "", "", false)
	if f != nil || q.Source != "processor" || len(stripe.locks) != 1 {
		t.Fatalf("stripe: %+v %v", q, f)
	}
	p.FXSource = routing.FXReference
	if q, f = quoteVia(ctx, reg, p, processor.Stripe, 800, "IN", "", "", false); f != nil || q.Source != "reference" || q.Amount != 75400 || len(stripe.locks) != 1 {
		t.Fatalf("reference: %+v %v", q, f)
	}
}

// A policy stored before processors and accounts held its split in columns. It is read
// once into the processors it meant, rewritten in the new shape, and read the same
// afterwards; its saved versions are read the same way, and restore as the new shape.
func TestAPolicyStoredInTheOldShapeIsMigratedOnceAtLoad(t *testing.T) {
	ctx := heldStore(t)
	useRails(t, true, true)
	scope := "legacy-" + t.Name()
	old := routingpolicy.Row{SquareWeight: 70, SquareOn: true, StripeOn: true, PayPalOn: true, CryptoOn: false, LocalOn: true,
		Rules: `{"fallback":["stripe"],"noSticky":true}`, Version: 3, By: "z@hanzo.ai"}
	if err := routingpolicy.Put(ctx, scope, old); err != nil {
		t.Fatal(err)
	}
	want := []routing.ProcessorRule{{Processor: "square", On: true, Weight: 70}, {Processor: "stripe", On: true, Weight: 30}, {Processor: "paypal", On: true}, {Processor: "crypto"}}
	p, v, err := ReadRoutingVersioned(ctx, scope)
	if err != nil || v != 3 || !reflect.DeepEqual(p.Processors, want) || len(p.Fallback) != 1 || p.Sticky {
		t.Fatalf("migrated policy = %+v (v%d, %v); want %+v, the rules kept", p, v, err, want)
	}
	// Written back in the new shape: the columns are gone, the split is in the rules.
	row, found, _ := routingpolicy.Get(ctx, scope)
	if !found || row.SquareWeight != 0 || row.SquareOn || row.StripeOn || row.PayPalOn || !strings.Contains(row.Rules, `"processors"`) || row.Version != 3 || row.By != "z@hanzo.ai" {
		t.Fatalf("row after the migration = %+v; want the new shape with its version and author", row)
	}
	again, _, _ := ReadRoutingVersioned(ctx, scope)
	if !reflect.DeepEqual(again.Processors, want) {
		t.Fatalf("a second read = %+v", again.Processors)
	}

	// A version saved in the old shape reads as the processors it meant, and restores as
	// a new version in the new shape.
	if err := routingpolicy.PutVersion(ctx, routingpolicy.Version{Scope: scope, N: 1, By: "z", Note: "old",
		Policy: `{"squareWeight":20,"squareOn":true,"stripeOn":true,"paypalOn":true,"cryptoOn":true,"localOn":true,"currencyRules":[],"countryRules":[],"brandRules":[],"issuerCountryRules":[],"fallback":[],"methods":[],"fxSpreadBps":200,"localCurrencies":[]}`}); err != nil {
		t.Fatal(err)
	}
	vs, _ := ListRoutingVersions(ctx, scope)
	if len(vs) != 1 || squareWeightOf(vs[0].Policy) != 20 || len(vs[0].Policy.Processors) != 4 {
		t.Fatalf("the old version reads %+v", vs)
	}
	restored, f := RestoreRouting(ctx, scope, 1, "z", "")
	if f != nil || squareWeightOf(restored) != 20 || restored.Processors[1].Weight != 80 {
		t.Fatalf("restore = %+v %v; want Square 20, Stripe 80", restored, f)
	}
	if row, _, _ := routingpolicy.Get(ctx, scope); !strings.Contains(row.Rules, `"processors"`) || row.SquareWeight != 0 {
		t.Fatalf("the restored row is in the old shape: %+v", row)
	}
}

func TestCryptoIsOfferedOnlyWhenTheCryptoMenuCanServe(t *testing.T) {
	ctx := heldStore(t)
	org := routingOrg(ctx, t)
	t.Setenv(processor.ChargeProcessors, "stripe,square,paypal")
	old, oldServes := processorsForOrg, cryptoServes
	processorsForOrg = func(*organization.Organization) *processor.Registry {
		reg := processor.NewRegistry(processor.DefaultConfig())
		for _, k := range []processor.ProcessorType{processor.Square, processor.Stripe, processor.PayPal} {
			reg.Register(newRail(k, true))
		}
		return reg
	}
	t.Cleanup(func() { processorsForOrg, cryptoServes = old, oldServes })

	// The picker cannot serve (custody signer not allowed or unreachable): crypto is not offered.
	cryptoServes = func(context.Context) bool { return false }
	if d, f := Place(ctx, org, RouteIn{Subject: "c", Country: "DE"}); f != nil || contains(d.Alternatives, "crypto") {
		t.Fatalf("crypto was offered while the menu cannot serve: %+v %v", d, f)
	}
	// The picker can serve: crypto is offered beside the card.
	cryptoServes = func(context.Context) bool { return true }
	if d, f := Place(ctx, org, RouteIn{Subject: "c", Country: "DE"}); f != nil || !contains(d.Alternatives, "crypto") {
		t.Fatalf("crypto was not offered while the menu can serve: %+v %v", d, f)
	}
	// The real answer with the custody processors not allowed by the charge policy is "cannot serve".
	cryptoServes = oldServes
	if cryptoServes(ctx) {
		t.Fatal("crypto served with mpc and kraken outside COMMERCE_CHARGE_PROCESSORS")
	}
}

func contains(l []string, s string) bool {
	for _, v := range l {
		if v == s {
			return true
		}
	}
	return false
}

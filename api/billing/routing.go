package billing

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/billing/engine"
	"github.com/hanzoai/commerce/checkout"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/paymentmethod"
	"github.com/hanzoai/commerce/models/routingpolicy"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment"
	"github.com/hanzoai/commerce/payment/processor"
	squareprovider "github.com/hanzoai/commerce/payment/providers/square"
	"github.com/hanzoai/commerce/payment/routing"
	jsonhttp "github.com/hanzoai/commerce/util/json/http"
)

// ReadRouting is the policy an org's new card checkouts are divided by: the org's
// own row, else the platform's, else the default (half and half across Square's
// eight countries).
func ReadRouting(ctx context.Context, org string) (routing.Policy, error) {
	for _, id := range []string{org, routingpolicy.Platform} {
		if id == "" {
			continue
		}
		r, found, err := routingpolicy.Get(ctx, id)
		if err != nil {
			return routing.Policy{}, err
		}
		if found {
			return routing.Policy{
				SquareWeight: r.SquareWeight, SquareOn: r.SquareOn, StripeOn: r.StripeOn, LocalOn: r.LocalOn,
				PayPalOn: r.PayPalOn, CryptoOn: r.CryptoOn,
			}.Normal(), nil
		}
	}
	return routing.Default(), nil
}

// WriteRouting stores p as the policy for scope (an org name, or "" for the platform
// default) and records the change in the billing events, with who made it and the
// policy it replaced. WHO MAY CALL IT is not decided here: the endpoint sits behind
// the platform bar (owner == "admin"), which an org's own admin does not clear.
func WriteRouting(ctx context.Context, scope string, p routing.Policy, by string) (routing.Policy, *PaymentFault) {
	p = p.Normal()
	if why := p.Valid(); why != "" {
		return p, fault(400, why, errors.New(why))
	}
	id := strings.ToLower(strings.TrimSpace(scope))
	if id == "" {
		id = routingpolicy.Platform
	}
	before, err := ReadRouting(ctx, id)
	if err != nil {
		return p, fault(503, "routing is temporarily unavailable; please retry", err)
	}
	if err := routingpolicy.Put(ctx, id, routingpolicy.Row{
		SquareWeight: p.SquareWeight, SquareOn: p.SquareOn, StripeOn: p.StripeOn, LocalOn: p.LocalOn,
		PayPalOn: p.PayPalOn, CryptoOn: p.CryptoOn,
		By: by,
	}); err != nil {
		return p, fault(503, "routing could not be stored", err)
	}
	// The audit record is part of the act: a change nobody can read back did not
	// happen as far as an auditor is concerned, so a record that cannot be written
	// fails the request (the policy is put back).
	if _, err := engine.EmitBillingEvent(webhookGuardDB(ctx), "routing.changed", "routing", id, "",
		map[string]interface{}{"policy": p, "by": by}, map[string]interface{}{"policy": before}); err != nil {
		_ = routingpolicy.Put(ctx, id, routingpolicy.Row{
			SquareWeight: before.SquareWeight, SquareOn: before.SquareOn, StripeOn: before.StripeOn, LocalOn: before.LocalOn,
			PayPalOn: before.PayPalOn, CryptoOn: before.CryptoOn,
			By: "revert",
		})
		return p, fault(503, "the change could not be audited, so it was not made", err)
	}
	return p, nil
}

// attemptHold is how long a Square charge of unknown outcome keeps its payer from
// being offered another processor.
const attemptHold = 15 * time.Minute

// RouteIn is one buyer asking where to pay.
type RouteIn struct {
	Subject string
	Country string
	// Currency is what the checkout is paid in; empty is usd.
	Currency string
}

// savedProcessor is the processor a card the subject saved was vaulted with: the
// default card's, else the newest card's. "" for a subject with none.
func savedProcessor(ctx context.Context, org *organization.Organization, subject string) processor.ProcessorType {
	db := datastore.New(org.Namespaced(ctx))
	rows := make([]*paymentmethod.PaymentMethod, 0)
	q := paymentmethod.Query(db).Ancestor(db.NewKey("synckey", "", 1, nil)).Filter("CustomerId=", subject).Filter("Type=", "card")
	if _, err := q.Order("-Created").GetAll(&rows); err != nil || len(rows) == 0 {
		return ""
	}
	for _, r := range rows {
		if r.IsDefault {
			return providerOf(r)
		}
	}
	return providerOf(rows[0])
}

// Place says which processor takes a NEW checkout for the buyer and which methods
// to mount. A buyer with a saved card stays on that card's processor and is never
// moved; a processor that is down (unregistered, unconfigured, denied by the
// deployment's charge policy) hands a NEW buyer to the other one, for this checkout
// only.
func Place(ctx context.Context, org *organization.Organization, in RouteIn) (*routing.Decision, *PaymentFault) {
	if org == nil || in.Subject == "" {
		return nil, fault(401, "sign in to check out", nil)
	}
	country := strings.ToUpper(strings.TrimSpace(in.Country))
	if processor.Sanctioned(country) {
		return nil, fault(403, "payments are not available here", nil)
	}
	pol, err := ReadRouting(ctx, org.Name)
	if err != nil {
		return nil, fault(503, "routing is temporarily unavailable; please retry", err)
	}
	saved := savedProcessor(ctx, org, in.Subject)
	reg := processorsForOrg(org)
	cur := currency.Type(strings.ToLower(strings.TrimSpace(in.Currency)))
	if cur == "" {
		cur = currency.USD
	}
	// A Square charge begun for this payer and not yet known to have ended holds them
	// where they are: no other processor is offered until it is known whether the
	// first one took the money. (A hold older than attemptHold is let go.)
	asg, assigned, aerr := routingpolicy.Assigned(ctx, org.Name, in.Subject)
	if aerr != nil {
		return nil, fault(503, "routing is temporarily unavailable; please retry", aerr)
	}
	if assigned && asg.Attempt != "" && time.Since(asg.AttemptAt) < attemptHold {
		return nil, fault(409, "a payment is still being confirmed; wait for it to finish before paying again", nil)
	}
	// The merchant account decides first: Square takes this checkout only if a Square
	// location of ours can charge a card in this currency. The buyer's country is not
	// asked.
	squareOK := false
	if sq, err := reg.Registered(processor.Square); err == nil && sq.IsAvailable(ctx) {
		if m, ok := sq.(processor.Merchant); ok {
			if _, eligible := m.Eligible(ctx, cur); eligible {
				squareOK = true
			}
		}
	}
	var prior processor.ProcessorType
	if assigned {
		prior = processor.ProcessorType(asg.Processor)
	}
	d := pol.Decide(routing.Input{Org: org.Name, Key: in.Subject, Country: country, Saved: saved, Assigned: prior, SquareOK: squareOK})
	if d.Reason == "split" {
		// First placement by the split, kept: the weight changed later moves nobody
		// who has been placed.
		if err := routingpolicy.Place(ctx, org.Name, in.Subject, string(d.Processor)); err != nil {
			log.Warn("routing: the placement was not kept: %v", err)
		}
	}

	take := func(t processor.ProcessorType) bool {
		_, err := reg.Fiat(ctx, processor.Need{Currency: cur, Method: "card", Want: t})
		return err == nil
	}
	if !take(d.Processor) {
		if saved != "" {
			// A saved card is charged where it was saved or not at all.
			return nil, fault(503, "your saved card's processor is unavailable right now; please retry shortly", nil)
		}
		other := processor.Square
		if d.Processor == processor.Square {
			other = processor.Stripe
		}
		if (other == processor.Square && !pol.SquareOn) || (other == processor.Stripe && !pol.StripeOn) || !take(other) {
			return nil, fault(503, "no payment processor is available right now", nil)
		}
		d.Processor, d.Reason, d.Methods = other, "fallback", pol.Methods(other, country)
	}
	d.Alternatives = alternativesFor(ctx, reg, pol, country)
	if err := routingpolicy.Record(ctx, org.Name, string(d.Processor), country, d.Reason); err != nil {
		log.Warn("routing: the decision was not recorded: %v", err)
	}
	return &d, nil
}

// alternativesFor are the other ways a buyer in country may pay, beyond the card
// slot: PayPal wherever PayPal operates and the deployment allows it and the policy
// has it on, and native crypto where a deposit watcher is really running.
func alternativesFor(ctx context.Context, reg *processor.Registry, pol routing.Policy, country string) []string {
	alt := []string{}
	if pol.PayPalOn && payment.IsPayPalCountry(country) && !processor.DisabledByPolicy(processor.PayPal) {
		if p, err := reg.Registered(processor.PayPal); err == nil && p.IsAvailable(ctx) {
			alt = append(alt, "paypal")
		}
	}
	if pol.CryptoOn && len(watchedAssets()) > 0 {
		alt = append(alt, "crypto")
	}
	return alt
}

// RoutingView is what the platform admin reads: the policy, the platform default,
// and where the latest checkouts went.
type RoutingView struct {
	Policy    routing.Policy   `json:"policy"`
	Default   routing.Policy   `json:"default"`
	Counts    []RoutingCount   `json:"counts"`
	Recent    []RoutingRecent  `json:"recent"`
	Processor []ProcessorState `json:"processors"`
	// Locations are the Square merchant locations, read from Square: their country,
	// currency, capabilities and the methods each can mount. Country is information
	// about the location, never a condition on a buyer.
	Locations []squareprovider.Place `json:"locations"`
}

// RoutingCount is how many of the recent decisions each processor took.
type RoutingCount struct {
	Processor string `json:"processor"`
	Count     int    `json:"count"`
}

// RoutingRecent is one recent decision.
type RoutingRecent struct {
	At        string `json:"at"`
	Org       string `json:"org"`
	Processor string `json:"processor"`
	Country   string `json:"country"`
	Reason    string `json:"reason"`
}

// ProcessorState says whether a processor can take a payment here now.
type ProcessorState struct {
	Processor string `json:"processor"`
	Available bool   `json:"available"`
}

// ReadRoutingView assembles the admin's page for scope (an org, or "" for the platform).
func ReadRoutingView(ctx context.Context, org *organization.Organization, scope string) (*RoutingView, *PaymentFault) {
	pol, err := ReadRouting(ctx, scope)
	if err != nil {
		return nil, fault(503, "routing is temporarily unavailable", err)
	}
	rows, err := routingpolicy.Recent(ctx, 500)
	if err != nil {
		return nil, fault(503, "routing decisions could not be read", err)
	}
	counts := map[string]int{}
	recent := make([]RoutingRecent, 0, 50)
	for i, r := range rows {
		counts[r.Processor]++
		if i < 50 {
			recent = append(recent, RoutingRecent{At: r.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"), Org: r.Org, Processor: r.Processor, Country: r.Country, Reason: r.Reason})
		}
	}
	v := &RoutingView{Policy: pol, Default: routing.Default(), Recent: recent,
		Counts: []RoutingCount{{"square", counts["square"]}, {"stripe", counts["stripe"]}}}
	reg := processorsForOrg(orgOrZero(org))
	for _, t := range []processor.ProcessorType{processor.Square, processor.Stripe, processor.PayPal} {
		_, ferr := reg.Fiat(ctx, processor.Need{Currency: currency.USD, Method: "card", Want: t})
		ok := ferr == nil
		if t == processor.PayPal {
			p, perr := reg.Registered(t)
			ok = perr == nil && p.IsAvailable(ctx) && !processor.DisabledByPolicy(t)
		}
		v.Processor = append(v.Processor, ProcessorState{string(t), ok})
	}
	v.Processor = append(v.Processor, ProcessorState{"crypto", len(watchedAssets()) > 0})
	v.Locations = []squareprovider.Place{}
	if sq, err := reg.Registered(processor.Square); err == nil {
		if lister, ok := sq.(interface {
			Locations(context.Context) ([]squareprovider.Place, error)
		}); ok {
			if places, lerr := lister.Locations(ctx); lerr != nil {
				log.Warn("routing: the Square locations could not be listed: %v", lerr)
			} else {
				v.Locations = places
			}
		}
	}
	return v, nil
}

// requestCountry is the buyer's country from the edge's geo headers, "" when none.
func requestCountry(c *zip.Ctx) string { return checkout.RequestCountry(c) }

type routeRequest struct {
	Country  string `json:"country"`
	Currency string `json:"currency"`
}

// RouteCheckout answers a signed-in buyer which processor takes their checkout.
//
//	POST /v1/billing/route   { country? }
func RouteCheckout(c *zip.Ctx) error {
	org, ok := middleware.GetOrganizationOK(c)
	if !ok || org == nil {
		return jsonhttp.Fail(c, 401, "sign in to check out", nil)
	}
	var req routeRequest
	_ = c.Bind(&req)
	country := strings.TrimSpace(req.Country)
	if country == "" {
		country = requestCountry(c)
	}
	d, f := Place(c.Context(), org, RouteIn{Subject: userBillingKey(c), Country: country, Currency: req.Currency})
	if f != nil {
		return jsonhttp.Fail(c, f.Status, f.Message, f.Err)
	}
	return c.JSON(200, d)
}

type routingScope struct {
	Org string `json:"org"`
}

// GetRouting is the platform admin's read of the routing policy.
//
//	GET /v1/billing/routing?org=
func GetRouting(c *zip.Ctx) error {
	org, _ := middleware.GetOrganizationOK(c)
	v, f := ReadRoutingView(c.Context(), org, strings.TrimSpace(c.Query("org")))
	if f != nil {
		return jsonhttp.Fail(c, f.Status, f.Message, f.Err)
	}
	return c.JSON(200, v)
}

type routingWrite struct {
	routingScope
	routing.Policy
}

// PutRouting sets the routing policy for an org, or for the platform when no org
// is named. Mounted on the platform-only group: an org's own admin is refused.
//
//	PUT /v1/billing/routing   { org?, squareWeight, squareOn, stripeOn, localOn, squareCountries }
func PutRouting(c *zip.Ctx) error {
	var req routingWrite
	if err := c.Bind(&req); err != nil {
		return jsonhttp.Fail(c, 400, "invalid request body", err)
	}
	p, f := WriteRouting(c.Context(), req.Org, req.Policy, middleware.Actor(c))
	if f != nil {
		return jsonhttp.Fail(c, f.Status, f.Message, f.Err)
	}
	return c.JSON(200, p)
}

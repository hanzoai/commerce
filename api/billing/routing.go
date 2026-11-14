package billing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
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
	"github.com/hanzoai/commerce/payment/fx"
	"github.com/hanzoai/commerce/payment/processor"
	squareprovider "github.com/hanzoai/commerce/payment/providers/square"
	"github.com/hanzoai/commerce/payment/routing"
	jsonhttp "github.com/hanzoai/commerce/util/json/http"
)

// rules is the part of a policy the store keeps as one JSON string (it holds no lists).
type rules struct {
	CurrencyRules      []routing.CurrencyRule      `json:"currencyRules"`
	CountryRules       []routing.CountryRule       `json:"countryRules"`
	BrandRules         []routing.BrandRule         `json:"brandRules"`
	IssuerCountryRules []routing.IssuerRule        `json:"issuerCountryRules"`
	Fallback           []string                    `json:"fallback"`
	LocalMethods       []routing.MethodRule        `json:"methods"`
	NoSticky           bool                        `json:"noSticky,omitempty"`
	LocalCurrencies    []routing.LocalCurrencyRule `json:"localCurrencies"`
	FXSource           string                      `json:"fxSource,omitempty"`
	FXCost             string                      `json:"fxCost,omitempty"`
	NoRounding         bool                        `json:"noRounding,omitempty"`
	RoundingIncrements []routing.RoundingIncrement `json:"roundingIncrements"`
}

func encodeRules(p routing.Policy) string {
	b, _ := json.Marshal(rules{p.CurrencyRules, p.CountryRules, p.BrandRules, p.IssuerCountryRules, p.Fallback, p.LocalMethods, !p.Sticky, p.LocalCurrencies, p.FXSource, p.FXCost, p.NoRounding, p.RoundingIncrements})
	return string(b)
}

func policyOf(r *routingpolicy.Row) routing.Policy {
	p := routing.Default()
	p.SquareWeight, p.SquareOn, p.StripeOn, p.LocalOn = r.SquareWeight, r.SquareOn, r.StripeOn, r.LocalOn
	p.PayPalOn, p.CryptoOn = r.PayPalOn, r.CryptoOn
	if strings.TrimSpace(r.Rules) != "" {
		var x rules
		if json.Unmarshal([]byte(r.Rules), &x) == nil {
			p.CurrencyRules, p.CountryRules, p.BrandRules, p.IssuerCountryRules, p.Fallback, p.LocalMethods, p.Sticky = x.CurrencyRules, x.CountryRules, x.BrandRules, x.IssuerCountryRules, x.Fallback, x.LocalMethods, !x.NoSticky
			p.LocalCurrencies, p.FXSource, p.FXCost, p.NoRounding, p.RoundingIncrements = x.LocalCurrencies, x.FXSource, x.FXCost, x.NoRounding, x.RoundingIncrements
		}
	}
	return p.Normal()
}

func policyRow(p routing.Policy, by string, version int) routingpolicy.Row {
	return routingpolicy.Row{
		SquareWeight: p.SquareWeight, SquareOn: p.SquareOn, StripeOn: p.StripeOn, LocalOn: p.LocalOn,
		PayPalOn: p.PayPalOn, CryptoOn: p.CryptoOn, Rules: encodeRules(p), By: by, Version: version,
	}
}

// ReadRouting is the policy an org's new card checkouts are divided by: the org's
// own row, else the platform's, else the default (half and half, sticky, every
// method on, no rules).
func ReadRouting(ctx context.Context, org string) (routing.Policy, error) {
	p, _, err := ReadRoutingVersioned(ctx, org)
	return p, err
}

// ReadRoutingVersioned is ReadRouting with the version of the policy it read, so a
// decision, an intent and an attempt can say which policy they were made under.
func ReadRoutingVersioned(ctx context.Context, org string) (routing.Policy, int, error) {
	for _, id := range []string{org, routingpolicy.Platform} {
		if id == "" {
			continue
		}
		r, found, err := routingpolicy.Get(ctx, id)
		if err != nil {
			return routing.Policy{}, 0, err
		}
		if found {
			return policyOf(r), r.Version, nil
		}
	}
	return routing.Default(), 0, nil
}

// activeVersion is the version the scope's policy row points at, 0 for none.
func activeVersion(ctx context.Context, scope string) int {
	if r, found, err := routingpolicy.Get(ctx, scope); err == nil && found {
		return r.Version
	}
	return 0
}

// registryFor is the processors a policy for scope is checked against: the scoped
// org's own, else the deployment's.
func registryFor(ctx context.Context, scope string) *processor.Registry {
	if scope != "" && scope != routingpolicy.Platform {
		if org, err := intentOrg(ctx, scope, false); err == nil {
			return processorsForOrg(org)
		}
	}
	return processorsForOrg(&organization.Organization{})
}

// quoteFor is what a payment of amountCents dollars of credit costs in the currency the
// buyer chose (explicit), else their country's default, disclosed before they
// authorize. It refuses, with a clear 400, a currency that is not viable: not one
// Stripe takes, or not one the chosen method can be paid in (Pix in dollars), or an
// amount under Stripe's minimum charge. The country table only ever supplies the
// default. Amounts are in the currency's own minor unit (yen, fils, cents), rounded up
// to the policy's increment. bind says a payment is about to be opened on this quote:
// the processor's rate is then locked for it, and the quote names the lock.
func quoteFor(ctx context.Context, reg *processor.Registry, pol routing.Policy, amountCents int64, country, explicit, method string, bind bool) (*routing.Quote, *PaymentFault) {
	cur := strings.ToLower(strings.TrimSpace(explicit))
	if cur == "" {
		cur = pol.LocalCurrency(country)
		if method != "" && !routing.MethodViable(method, cur) {
			return nil, fault(400, method+" is paid in "+strings.Join(routing.MethodCurrencies[strings.ToLower(method)], " or ")+", not "+cur+"; choose one of them", nil)
		}
	}
	if !routing.MethodViable(method, cur) {
		why := method + " cannot be paid in " + cur + "; it is paid in " + strings.Join(routing.MethodCurrencies[strings.ToLower(method)], " or ")
		return nil, fault(400, why, errors.New(why))
	}
	if cur == "usd" {
		return &routing.Quote{Amount: amountCents, Decimals: 2, Currency: "usd", UsdCents: amountCents, Rate: 1}, nil
	}
	if amountCents < fx.MinChargeUSDCents {
		return nil, fault(400, fmt.Sprintf("Stripe takes no charge under %d usd cents (its minimum, which a charge in %s must also meet)", fx.MinChargeUSDCents, cur), nil)
	}
	table, _ := eligibility(ctx, reg)
	stripeTakes := false
	for _, e := range table {
		if e.Currency == cur {
			for _, x := range e.Processors {
				stripeTakes = stripeTakes || x == "stripe"
			}
		}
	}
	if !stripeTakes {
		return nil, fault(400, "stripe cannot take "+cur+" for this account", nil)
	}
	step := pol.Step(cur)
	dec := int(currency.Type(cur).Decimals())
	// Stripe's FX quote first. Priced "pass" (the default) the rate is its effective
	// exchange_rate, fee and lock premium in it, so the dollars credited settle whole;
	// priced "absorb" it is base_rate and Hanzo bears the cost. No percentage of Hanzo's
	// is added to either. The market rate is the fallback when the account cannot use
	// the quote, and Hanzo bears the conversion cost there: no fee is guessed.
	if pol.FXSource != routing.FXMarket {
		if rq, err := processorRate(ctx, reg, cur, bind, method); err == nil {
			rate, cost := rq.PerDollar, routing.FXPass
			if pol.FXCost == routing.FXAbsorb {
				rate, cost = rq.BasePerDollar, routing.FXAbsorb
			}
			if paid, cerr := fx.Convert(amountCents, cur, rate, step); cerr == nil {
				q := &routing.Quote{Amount: paid, Decimals: dec, Currency: cur, UsdCents: amountCents, Rate: rate,
					Source: routing.FXProcessor, FXCost: cost, ExchangeRate: rq.ExchangeRate, BaseRate: rq.BasePerDollar,
					FeeRate: rq.FeeRate, Premium: rq.Premium, LockExpires: rq.ExpiresAt, Increment: step, Disclosure: routing.Disclosure}
				if bind {
					q.FxQuote = rq.ID
					q.ReferenceRate, q.ReferenceProvider = rq.ReferenceRate, rq.ReferenceProvider
				}
				return q, nil
			}
		} else {
			log.Warn("fx: Stripe's FX quote for %s is unavailable, using the market rate: %v", cur, err)
		}
	}
	paid, rate, err := fx.Quote(ctx, amountCents, cur, step)
	if err != nil {
		return nil, fault(503, "local payment methods are unavailable right now; please retry", err)
	}
	return &routing.Quote{Amount: paid, Decimals: dec, Currency: cur, UsdCents: amountCents, Rate: rate,
		Source: routing.FXMarket, FXCost: routing.FXAbsorb, Increment: step, Disclosure: routing.Disclosure}, nil
}

// displayTTL is how long a processor rate shown as a price is reused: an hour, well
// inside the day a displayed price may go without a refresh. A price a payment is bound
// to is never taken from here.
const displayTTL = time.Hour

type shownRate struct {
	q  *processor.RateQuote
	at time.Time
}

var (
	shownMu sync.Mutex
	shown   = map[string]shownRate{}
)

// processorRate is the processor's rate for cur: a fresh locked quote when a payment is
// being bound to it (lock for the life of the method: a day for a voucher or a bank
// transfer that settles after the buyer leaves, an hour otherwise), else the cached
// current rate.
func processorRate(ctx context.Context, reg *processor.Registry, cur string, bind bool, method string) (*processor.RateQuote, error) {
	sp, err := reg.Registered(processor.Stripe)
	if err != nil {
		return nil, err
	}
	rq, ok := sp.(processor.RateQuoter)
	if !ok {
		return nil, errors.New("this processor does not quote rates")
	}
	if bind {
		lock := processor.LockHour
		switch strings.ToLower(method) {
		case "pix", "boleto", "oxxo", "sepa_debit":
			lock = processor.LockDay
		}
		return rq.QuoteRate(ctx, cur, lock)
	}
	shownMu.Lock()
	if c, ok := shown[cur]; ok && time.Since(c.at) < displayTTL {
		shownMu.Unlock()
		return c.q, nil
	}
	shownMu.Unlock()
	q, err := rq.QuoteRate(ctx, cur, processor.LockNone)
	if err != nil {
		return nil, err
	}
	shownMu.Lock()
	shown[cur] = shownRate{q, time.Now()}
	shownMu.Unlock()
	return q, nil
}

// CurrencyEligibility says which card processors can take a currency now. It is the
// real answer: Square's is the currency of its location (when it holds card
// processing), Stripe's is the currencies it supports.
type CurrencyEligibility struct {
	Currency   string   `json:"currency"`
	Processors []string `json:"processors"`
}

func eligibility(ctx context.Context, reg *processor.Registry) ([]CurrencyEligibility, bool) {
	by := map[string][]string{}
	squareAny := false
	if sq, err := reg.Registered(processor.Square); err == nil && sq.IsAvailable(ctx) {
		for _, c := range sq.SupportedCurrencies() {
			by[string(c)] = append(by[string(c)], "square")
			squareAny = true
		}
	}
	if st, err := reg.Registered(processor.Stripe); err == nil && st.IsAvailable(ctx) {
		for _, c := range st.SupportedCurrencies() {
			by[string(c)] = append(by[string(c)], "stripe")
		}
	}
	out := make([]CurrencyEligibility, 0, len(by))
	for c, ps := range by {
		out = append(out, CurrencyEligibility{c, ps})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Currency < out[j].Currency })
	return out, squareAny
}

// ValidateRouting refuses a policy that routes to a processor that cannot take the
// charge: a currency rule needs a processor that takes that currency now; a brand, an
// issuer-country or a fallback entry naming Square needs a Square location that holds
// card processing; and names that are not a brand, country, currency or method are
// refused. It says why, or "".
func ValidateRouting(ctx context.Context, scope string, p routing.Policy) string {
	if why := p.Valid(); why != "" {
		return why
	}
	table, squareAny := eligibility(ctx, registryFor(ctx, scope))
	takes := func(cur, proc string) bool {
		for _, e := range table {
			if e.Currency == cur {
				for _, x := range e.Processors {
					if x == proc {
						return true
					}
				}
			}
		}
		return false
	}
	for _, r := range p.LocalCurrencies {
		if r.Currency == "usd" {
			continue // usd switches local currency off for the country
		}
		stripeTakes := false
		for _, e := range table {
			if e.Currency == r.Currency {
				for _, x := range e.Processors {
					stripeTakes = stripeTakes || x == "stripe"
				}
			}
		}
		if !stripeTakes {
			return "localCurrencies: stripe cannot take " + r.Currency + " now"
		}
		qctx, cancel := context.WithTimeout(ctx, 6*time.Second)
		_, _, qerr := fx.Quote(qctx, 100, r.Currency, fx.Granule(r.Currency))
		cancel()
		if qerr != nil {
			return "localCurrencies: " + r.Currency + " has no rate source, so it cannot be quoted (" + qerr.Error() + ")"
		}
	}
	for _, r := range p.CurrencyRules {
		if !takes(r.Currency, r.Processor) {
			return "currencyRules: " + r.Processor + " cannot take " + r.Currency + " now (its merchant account does not)"
		}
	}
	for _, r := range p.BrandRules {
		if r.Processor == "square" && !squareAny {
			return "brandRules: square has no merchant location that takes cards, so " + r.Brand + " cannot go there"
		}
	}
	for _, r := range p.CountryRules {
		if r.Processor == "square" && !squareAny {
			return "countryRules: square has no merchant location that takes cards, so buyers in " + r.Country + " cannot go there"
		}
	}
	for _, r := range p.IssuerCountryRules {
		if r.Processor == "square" && !squareAny {
			return "issuerCountryRules: square has no merchant location that takes cards, so cards issued in " + r.Country + " cannot go there"
		}
	}
	for _, f := range p.Fallback {
		if f == "square" && !squareAny {
			return "fallback: square has no merchant location that takes cards"
		}
	}
	return ""
}

// WriteRouting stores p as the policy for scope (an org name, or "" for the platform
// default) and records the change in the billing events, with who made it and the
// policy it replaced. WHO MAY CALL IT is not decided here: the endpoint sits behind
// the platform bar (owner == "admin"), which an org's own admin does not clear.
func WriteRouting(ctx context.Context, scope string, p routing.Policy, by, note string) (routing.Policy, *PaymentFault) {
	return saveRouting(ctx, scope, p, by, note, 0)
}

// saveRouting validates p against what the merchant accounts can take, writes it as a
// NEW immutable version, and points the active row at it. restored names the version
// being restored, if any: a restore is a save, so history is never rewritten.
func saveRouting(ctx context.Context, scope string, p routing.Policy, by, note string, restored int) (routing.Policy, *PaymentFault) {
	p = p.Normal()
	id := strings.ToLower(strings.TrimSpace(scope))
	if id == "" {
		id = routingpolicy.Platform
	}
	// Refused before anything is stored: a rule that cannot be honoured changes nothing.
	if why := ValidateRouting(ctx, id, p); why != "" {
		_, _ = engine.EmitBillingEvent(webhookGuardDB(ctx), "routing.refused", "routing", id, "",
			map[string]interface{}{"policy": p, "by": by, "why": why}, nil)
		return p, fault(400, why, errors.New(why))
	}
	before, err := ReadRouting(ctx, id)
	if err != nil {
		return p, fault(503, "routing is temporarily unavailable; please retry", err)
	}
	prev := activeVersion(ctx, id)
	all, verr := routingpolicy.Versions(ctx, id)
	if verr != nil {
		return p, fault(503, "routing is temporarily unavailable; please retry", verr)
	}
	next := 1
	if len(all) > 0 {
		next = all[0].N + 1
	}
	pj, _ := json.Marshal(p)
	dj, _ := json.Marshal(diffPolicy(before, p))
	if err := routingpolicy.PutVersion(ctx, routingpolicy.Version{Scope: id, N: next, Policy: string(pj), By: by, Note: note, Diff: string(dj), Restored: restored}); err != nil {
		return p, fault(503, "the version could not be stored", err)
	}
	if err := routingpolicy.Put(ctx, id, policyRow(p, by, next)); err != nil {
		return p, fault(503, "routing could not be stored", err)
	}
	// The audit record is part of the act: a change nobody can read back did not
	// happen as far as an auditor is concerned, so a record that cannot be written
	// fails the request (the previous version stays active; the new one stays in
	// history, inactive).
	if _, err := engine.EmitBillingEvent(webhookGuardDB(ctx), "routing.changed", "routing", id, "",
		map[string]interface{}{"policy": p, "by": by, "note": note, "version": next, "restored": restored}, map[string]interface{}{"policy": before, "version": prev}); err != nil {
		_ = routingpolicy.Put(ctx, id, policyRow(before, "revert", prev))
		return p, fault(503, "the change could not be audited, so it was not made", err)
	}
	return p, nil
}

// diffPolicy lists the fields that differ, as "field: before -> after".
func diffPolicy(a, b routing.Policy) []string {
	var am, bm map[string]interface{}
	aj, _ := json.Marshal(a)
	bj, _ := json.Marshal(b)
	_ = json.Unmarshal(aj, &am)
	_ = json.Unmarshal(bj, &bm)
	keys := make([]string, 0, len(bm))
	for k := range bm {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := []string{}
	for _, k := range keys {
		x, _ := json.Marshal(am[k])
		y, _ := json.Marshal(bm[k])
		if string(x) != string(y) {
			out = append(out, k+": "+string(x)+" -> "+string(y))
		}
	}
	return out
}

// RoutingVersion is one saved version as the admin lists it.
type RoutingVersion struct {
	N        int            `json:"n"`
	Active   bool           `json:"active"`
	By       string         `json:"by"`
	At       string         `json:"at"`
	Note     string         `json:"note"`
	Diff     []string       `json:"diff"`
	Restored int            `json:"restored"`
	Policy   routing.Policy `json:"policy"`
}

// ListRoutingVersions lists the scope's history, newest first.
func ListRoutingVersions(ctx context.Context, scope string) ([]RoutingVersion, *PaymentFault) {
	id := strings.ToLower(strings.TrimSpace(scope))
	if id == "" {
		id = routingpolicy.Platform
	}
	rows, err := routingpolicy.Versions(ctx, id)
	if err != nil {
		return nil, fault(503, "the history could not be read", err)
	}
	active := activeVersion(ctx, id)
	out := make([]RoutingVersion, 0, len(rows))
	for _, v := range rows {
		var p routing.Policy
		_ = json.Unmarshal([]byte(v.Policy), &p)
		var d []string
		_ = json.Unmarshal([]byte(v.Diff), &d)
		if d == nil {
			d = []string{}
		}
		out = append(out, RoutingVersion{N: v.N, Active: v.N == active, By: v.By, At: v.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"), Note: v.Note, Diff: d, Restored: v.Restored, Policy: p.Normal()})
	}
	return out, nil
}

// RestoreRouting makes an earlier version current again, as a NEW version, after
// checking it still holds against what the merchant accounts can take today.
func RestoreRouting(ctx context.Context, scope string, n int, by, note string) (routing.Policy, *PaymentFault) {
	id := strings.ToLower(strings.TrimSpace(scope))
	if id == "" {
		id = routingpolicy.Platform
	}
	v, found, err := routingpolicy.GetVersion(ctx, id, n)
	if err != nil {
		return routing.Policy{}, fault(503, "the history could not be read", err)
	}
	if !found {
		return routing.Policy{}, fault(404, "no such version", nil)
	}
	var p routing.Policy
	if err := json.Unmarshal([]byte(v.Policy), &p); err != nil {
		return routing.Policy{}, fault(500, "that version cannot be read", err)
	}
	if note == "" {
		note = "restore of version " + strconv.Itoa(n)
	}
	return saveRouting(ctx, id, p, by, note, n)
}

// ResetRouting drops payers' placements: one subject's, or every payer's in org when
// subject is empty, so their next checkout is placed afresh. Audited.
func ResetRouting(ctx context.Context, org, subject, by string) (int, *PaymentFault) {
	org = strings.ToLower(strings.TrimSpace(org))
	if org == "" {
		return 0, fault(400, "org is required", nil)
	}
	n := 0
	if strings.TrimSpace(subject) != "" {
		if _, found, err := routingpolicy.Assigned(ctx, org, subject); err != nil {
			return 0, fault(503, "routing is temporarily unavailable; please retry", err)
		} else if found {
			n = 1
		}
		if err := routingpolicy.Forget(ctx, org, subject); err != nil {
			return 0, fault(503, "the placement could not be reset", err)
		}
	} else {
		var err error
		if n, err = routingpolicy.ForgetOrg(ctx, org); err != nil {
			return n, fault(503, "the placements could not be reset", err)
		}
	}
	if _, err := engine.EmitBillingEvent(webhookGuardDB(ctx), "routing.reset", "routing", org, "",
		map[string]interface{}{"org": org, "subject": subject, "placements": n, "by": by}, nil); err != nil {
		log.Error("routing: the reset of %s was made and its audit record was not: %v", org, err)
		return n, fault(503, "the reset was made and could not be audited", err)
	}
	return n, nil
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
	// Method is how the buyer chose to pay, when they have (card, cash_app, pix, ...).
	Method string
	// AmountCents, when given, asks for the quote of a local-currency payment of that
	// many dollars of credit, in QuoteCurrency (the buyer's choice) or the country's
	// default.
	AmountCents   int64
	QuoteCurrency string
	// Brand and IssuerCountry describe the card once it is entered; empty before.
	Brand, IssuerCountry string
}

// PreviewIn is a sample checkout the admin asks the effective route of. Subject is
// optional: with one, the sample is that payer (their saved card and placement
// count); without, a payer with neither.
type PreviewIn struct {
	Subject, Country, Currency, Method, Brand, IssuerCountry string
	AmountCents                                              int64
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
	return place(ctx, org, in, true)
}

// PreviewRouting answers what a sample checkout would be routed to and by which rule,
// changing nothing: no decision is recorded, no placement is kept.
func PreviewRouting(ctx context.Context, org *organization.Organization, in PreviewIn) (*routing.Decision, *PaymentFault) {
	subject := strings.TrimSpace(in.Subject)
	if subject == "" {
		subject = "preview"
	}
	return place(ctx, org, RouteIn{Subject: subject, Country: in.Country, Currency: in.Currency, QuoteCurrency: in.Currency, AmountCents: in.AmountCents, Method: in.Method, Brand: in.Brand, IssuerCountry: in.IssuerCountry}, false)
}

func place(ctx context.Context, org *organization.Organization, in RouteIn, live bool) (*routing.Decision, *PaymentFault) {
	if org == nil || in.Subject == "" {
		return nil, fault(401, "sign in to check out", nil)
	}
	country := strings.ToUpper(strings.TrimSpace(in.Country))
	if processor.Sanctioned(country) {
		return nil, fault(403, "payments are not available here", nil)
	}
	pol, polVersion, err := ReadRoutingVersioned(ctx, org.Name)
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
	if live && assigned && asg.Attempt != "" && time.Since(asg.AttemptAt) < attemptHold {
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
	squareWhy := ""
	if !squareOK {
		squareWhy = "no Square merchant location takes cards in " + string(cur)
		if sq, err := reg.Registered(processor.Square); err != nil || !sq.IsAvailable(ctx) {
			squareWhy = "Square is not configured"
		} else if lp, ok := sq.(interface {
			Location(context.Context) (squareprovider.Site, bool)
		}); ok {
			if site, ok := lp.Location(ctx); !ok || !site.Cards {
				squareWhy = "the Square location takes no cards"
			} else {
				squareWhy = "the Square location is " + strings.ToUpper(string(site.Currency)) + ", the checkout is " + strings.ToUpper(string(cur))
			}
		}
	}
	// Card rules apply only where card metadata is known before routing: the brand and
	// issuing country stored with a saved card. A new card is tokenized inside the
	// processor's own form, so nothing is known of it yet.
	brand, issuer := in.Brand, in.IssuerCountry
	if brand == "" && issuer == "" {
		brand, issuer = savedCardMeta(ctx, org, in.Subject)
	}
	d := pol.Decide(routing.Input{Org: org.Name, Key: in.Subject, Country: country, Saved: saved, Assigned: prior, SquareOK: squareOK, SquareWhy: squareWhy,
		Currency: string(cur), Method: in.Method, Brand: brand, IssuerCountry: issuer})
	if live && pol.Sticky && d.Reason == "split" {
		// First placement by the split, kept: the weight changed later moves nobody
		// who has been placed.
		if err := routingpolicy.Place(ctx, org.Name, in.Subject, string(d.Processor)); err != nil {
			log.Warn("routing: the placement was not kept: %v", err)
		}
	}

	take := func(t processor.ProcessorType) bool {
		if !pol.Usable(t, routing.Input{SquareOK: squareOK}) {
			return false
		}
		_, err := reg.Fiat(ctx, processor.Need{Currency: cur, Method: "card", Want: t})
		return err == nil
	}
	if d.Reason != "saved" && !take(d.Processor) || d.Reason == "saved" && !take2(ctx, reg, cur, d.Processor) {
		if saved != "" {
			// A saved card is charged where it was saved or not at all.
			return nil, fault(503, "your saved card's processor is unavailable right now; please retry shortly", nil)
		}
		// The fallback order, before any payment exists: the admin's list, else the
		// other card processor.
		order := pol.Fallback
		if len(order) == 0 {
			order = []string{"square", "stripe"}
		}
		picked := false
		for _, name := range order {
			t := processor.ProcessorType(name)
			if t == d.Processor || !take(t) {
				continue
			}
			d.Processor, d.Reason, d.Rule, d.Methods = t, "fallback", "fallback", pol.Methods(t, country)
			d.Chain[len(d.Chain)-1] = routing.Step{Step: "fallback", Applied: true, Why: "the chosen processor could not take the charge before payment began, so " + name + " (next in the fallback order)"}
			picked = true
			break
		}
		if !picked {
			return nil, fault(503, "no payment processor is available right now", nil)
		}
	}
	if in.AmountCents > 0 && d.Processor == processor.Stripe && d.Reason != "saved" && pol.LocalOn {
		q, qf := quoteFor(ctx, reg, pol, in.AmountCents, country, in.QuoteCurrency, in.Method, false)
		if qf != nil {
			return nil, qf
		}
		d.Quote = q
	}
	d.Alternatives = alternativesFor(ctx, reg, pol, country)
	d.LocalCurrency, d.Version = pol.LocalCurrency(country), polVersion
	if live {
		if err := routingpolicy.Record(ctx, org.Name, string(d.Processor), country, d.Reason, polVersion); err != nil {
			log.Warn("routing: the decision was not recorded: %v", err)
		}
	}
	return &d, nil
}

// savedCardMeta is the brand and issuing country stored with the payer's saved card:
// the only card metadata known before routing.
func savedCardMeta(ctx context.Context, org *organization.Organization, subject string) (string, string) {
	db := datastore.New(org.Namespaced(ctx))
	rows := make([]*paymentmethod.PaymentMethod, 0)
	q := paymentmethod.Query(db).Ancestor(db.NewKey("synckey", "", 1, nil)).Filter("CustomerId=", subject).Filter("Type=", "card")
	if _, err := q.Order("-Created").GetAll(&rows); err != nil || len(rows) == 0 {
		return "", ""
	}
	pm := rows[0]
	for _, r := range rows {
		if r.IsDefault {
			pm = r
		}
	}
	if pm.Card == nil {
		return "", ""
	}
	return strings.ToLower(pm.Card.Brand), strings.ToUpper(pm.Card.Country)
}

// take2 is whether a registry can charge a card in cur at processor t, ignoring the
// policy's switches: a saved card's processor is held to what it can charge.
func take2(ctx context.Context, reg *processor.Registry, cur currency.Type, t processor.ProcessorType) bool {
	_, err := reg.Fiat(ctx, processor.Need{Currency: cur, Method: "card", Want: t})
	return err == nil
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
	// Eligibility is which card processors can take which currency now, so a form
	// offers only rules that will be accepted. Brands and Methods are the names a rule
	// may use.
	Eligibility []CurrencyEligibility `json:"eligibility"`
	Brands      []string              `json:"brands"`
	Methods     []string              `json:"methods"`
	// MethodNotes say what a method's switch does and cannot do, for the admin to label it.
	MethodNotes []routing.MethodNote `json:"methodNotes"`
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
	v.Eligibility, _ = eligibility(ctx, registryFor(ctx, scope))
	v.Brands, v.Methods, v.MethodNotes = routing.Brands, routing.MethodNames, routing.MethodNotes
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
	Country string `json:"country"`
	// Currency is the currency the buyer chose to pay in, if they chose one; empty
	// takes the country's default. AmountCents asks for the quote.
	Currency    string `json:"currency"`
	AmountCents int64  `json:"amountCents"`
	// Method is how the buyer chose to pay, when they have. Card brand and issuing
	// country are not asked of the browser: a new card is tokenized inside the
	// processor's own form, and rules on them apply where the metadata is already known.
	Method string `json:"method"`
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
	d, f := Place(c.Context(), org, RouteIn{Subject: userBillingKey(c), Country: country, Currency: req.Currency, QuoteCurrency: req.Currency, AmountCents: req.AmountCents, Method: req.Method})
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
	Note string `json:"note"`
	routingScope
	routing.Policy
}

// PutRouting sets the routing policy for an org, or for the platform when no org
// is named. Mounted on the platform-only group: an org's own admin is refused.
//
//	PUT /v1/billing/routing   { org?, note?, <the policy> }
func PutRouting(c *zip.Ctx) error {
	// A field the body leaves out keeps its default (stickiness is on unless the admin
	// turns it off), so an older client's smaller body changes nothing it did not name.
	req := routingWrite{Policy: routing.Default()}
	if err := c.Bind(&req); err != nil {
		return jsonhttp.Fail(c, 400, "invalid request body", err)
	}
	p, f := WriteRouting(c.Context(), req.Org, req.Policy, middleware.Actor(c), req.Note)
	if f != nil {
		return jsonhttp.Fail(c, f.Status, f.Message, f.Err)
	}
	return c.JSON(200, p)
}

type previewRequest struct {
	Org           string `json:"org"`
	Subject       string `json:"subject"`
	Country       string `json:"country"`
	Currency      string `json:"currency"`
	Method        string `json:"method"`
	Brand         string `json:"brand"`
	IssuerCountry string `json:"issuerCountry"`
	AmountCents   int64  `json:"amountCents"`
}

// PreviewRoutingHandler answers the effective route of a sample checkout, and the rule
// that decided it, changing nothing.
//
//	POST /v1/billing/routing/preview   { org?, subject?, country, currency, method, brand, issuerCountry }
func PreviewRoutingHandler(c *zip.Ctx) error {
	var req previewRequest
	if err := c.Bind(&req); err != nil {
		return jsonhttp.Fail(c, 400, "invalid request body", err)
	}
	org, ok := middleware.GetOrganizationOK(c)
	if !ok || org == nil {
		return jsonhttp.Fail(c, 401, "sign in", nil)
	}
	if n := strings.TrimSpace(req.Org); n != "" && !strings.EqualFold(n, org.Name) {
		o, err := intentOrg(c.Context(), n, org.TestMode())
		if err != nil {
			return jsonhttp.Fail(c, 404, "org not found", err)
		}
		org = o
	}
	d, f := PreviewRouting(c.Context(), org, PreviewIn{Subject: req.Subject, Country: req.Country, Currency: req.Currency, Method: req.Method, Brand: req.Brand, IssuerCountry: req.IssuerCountry, AmountCents: req.AmountCents})
	if f != nil {
		return jsonhttp.Fail(c, f.Status, f.Message, f.Err)
	}
	return c.JSON(200, d)
}

type resetRequest struct {
	Org     string `json:"org"`
	Subject string `json:"subject"`
}

// ResetRoutingHandler drops payers' placements so their next checkout is placed afresh.
//
//	POST /v1/billing/routing/reset   { org, subject? }
func ResetRoutingHandler(c *zip.Ctx) error {
	var req resetRequest
	if err := c.Bind(&req); err != nil {
		return jsonhttp.Fail(c, 400, "invalid request body", err)
	}
	n, f := ResetRouting(c.Context(), req.Org, req.Subject, middleware.Actor(c))
	if f != nil {
		return jsonhttp.Fail(c, f.Status, f.Message, f.Err)
	}
	return c.JSON(200, map[string]int{"placements": n})
}

// ListRoutingVersionsHandler lists the policy's history, newest first.
//
//	GET /v1/billing/routing/versions?org=
func ListRoutingVersionsHandler(c *zip.Ctx) error {
	out, f := ListRoutingVersions(c.Context(), strings.TrimSpace(c.Query("org")))
	if f != nil {
		return jsonhttp.Fail(c, f.Status, f.Message, f.Err)
	}
	return c.JSON(200, map[string]interface{}{"versions": out})
}

type restoreRequest struct {
	Org     string `json:"org"`
	Version int    `json:"version"`
	Note    string `json:"note"`
}

// RestoreRoutingHandler makes an earlier version current as a new version.
//
//	POST /v1/billing/routing/restore   { org?, version, note? }
func RestoreRoutingHandler(c *zip.Ctx) error {
	var req restoreRequest
	if err := c.Bind(&req); err != nil {
		return jsonhttp.Fail(c, 400, "invalid request body", err)
	}
	p, f := RestoreRouting(c.Context(), req.Org, req.Version, middleware.Actor(c), req.Note)
	if f != nil {
		return jsonhttp.Fail(c, f.Status, f.Message, f.Err)
	}
	return c.JSON(200, p)
}

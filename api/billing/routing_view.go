package billing

import (
	"context"
	"sort"
	"strings"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/checkout"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/routingpolicy"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/payment/routing"
	jsonhttp "github.com/hanzoai/commerce/util/json/http"
)

func isCustodyOnlyProcessor(p processor.PaymentProcessor) bool {
	if p == nil {
		return false
	}
	if _, ok := p.(processor.ManagedAccountProcessor); ok {
		return true
	}
	return p.Type() == processor.MPC
}

func merchantCapabilities(ctx context.Context, reg *processor.Registry, cur currency.Type) []routing.Capability {
	var out []routing.Capability
	for _, rp := range reg.RegisteredProcessors() {
		m, ok := rp.(processor.Merchant)
		if !ok {
			continue
		}
		pt := rp.Type()
		label := string(pt)
		if len(label) > 0 {
			label = strings.ToUpper(label[:1]) + label[1:]
		}
		eligible := false
		if rp.IsAvailable(ctx) {
			_, eligible = m.Eligible(ctx, cur)
		}
		why := ""
		if !eligible {
			why = "no " + label + " merchant location takes cards in " + string(cur)
			if !rp.IsAvailable(ctx) {
				why = label + " is not configured"
			} else if lp, ok := rp.(processor.MerchantLocationLister); ok {
				if site, hasSite := lp.ActiveLocation(ctx); !hasSite || !site.Cards {
					why = "the " + label + " location takes no cards"
				} else {
					why = "the " + label + " location is " + strings.ToUpper(site.Currency) + ", the checkout is " + strings.ToUpper(string(cur))
				}
			}
		}
		out = append(out, routing.Capability{Processor: pt, OK: eligible, Why: why})
	}
	return out
}

// RoutingView is what the platform admin reads: the policy, the platform default,
// and where the latest checkouts went.
type RoutingView struct {
	Policy      routing.Policy        `json:"policy"`
	Default     routing.Policy        `json:"default"`
	Counts      []RoutingCount        `json:"counts"`
	Recent      []RoutingRecent       `json:"recent"`
	Processor   []ProcessorState      `json:"processors"`
	Accounts    []routing.Account     `json:"accounts"`
	Eligibility []CurrencyEligibility `json:"eligibility"`
	Brands      []string              `json:"brands"`
	Methods     []string              `json:"methods"`
	MethodNotes []routing.MethodNote  `json:"methodNotes"`
	Problems    []string              `json:"problems"`
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

// ProcessorState says what role a processor fills, whether it can take a payment now,
// and which currencies its live integration currently supports.
type ProcessorState struct {
	Processor  string   `json:"processor"`
	Available  bool     `json:"available"`
	Kind       string   `json:"kind,omitempty"`
	Currencies []string `json:"currencies,omitempty"`
}

func processorCurrencies(p interface{ SupportedCurrencies() []currency.Type }) []string {
	if p == nil {
		return nil
	}
	raw := p.SupportedCurrencies()
	if len(raw) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(raw))
	out := make([]string, 0, len(raw))
	for _, c := range raw {
		code := strings.ToLower(strings.TrimSpace(string(c)))
		if code == "" {
			continue
		}
		if _, dup := seen[code]; dup {
			continue
		}
		seen[code] = struct{}{}
		out = append(out, code)
	}
	sort.Strings(out)
	return out
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
	v := &RoutingView{Policy: pol, Default: routing.Default(), Recent: recent, Counts: []RoutingCount{}}
	reg := processorsForOrg(orgOrZero(org))
	for _, r := range pol.Processors {
		v.Counts = append(v.Counts, RoutingCount{r.Processor, counts[r.Processor]})
		t := processor.ProcessorType(r.Processor)
		var ok bool
		var kind string
		var curs []string
		switch t {
		case routing.CryptoRail:
			kind = "alternative"
			ok = cryptoServes(ctx)
		case processor.PayPal:
			kind = "alternative"
			if p, perr := reg.Registered(t); perr == nil {
				ok = p.IsAvailable(ctx)
				curs = processorCurrencies(p)
			}
		default:
			if p, perr := processor.Global().Registered(t); perr == nil && isCustodyOnlyProcessor(p) {
				kind = "custody"
				ok = p.IsAvailable(ctx)
				curs = processorCurrencies(p)
				break
			}
			kind = "card"
			if p, perr := reg.Registered(t); perr == nil {
				ok = p.IsAvailable(ctx)
				curs = processorCurrencies(p)
			}
		}
		v.Processor = append(v.Processor, ProcessorState{Processor: r.Processor, Available: ok, Kind: kind, Currencies: curs})
	}
	v.Eligibility, _, _ = eligibility(ctx, registryFor(ctx, scope))
	v.Brands, v.Methods, v.MethodNotes = routing.Brands, routing.MethodNames, routing.MethodNotes
	v.Problems = append(chargeProblems(ctx, pol), settlementProblems(ctx, pol)...)
	if v.Problems == nil {
		v.Problems = []string{}
	}
	v.Accounts = []routing.Account{}
	for _, rp := range reg.RegisteredProcessors() {
		if processor.DisabledByEnv(rp.Type()) {
			continue
		}
		lister, ok := rp.(processor.MerchantLocationLister)
		if !ok {
			continue
		}
		locs, lerr := lister.MerchantLocations(ctx)
		if lerr != nil {
			log.Warn("routing: %s merchant locations could not be listed: %v", rp.Type(), lerr)
			continue
		}
		for _, loc := range locs {
			v.Accounts = append(v.Accounts, routing.Account{
				Processor: string(rp.Type()),
				ID:        loc.ID,
				Country:   loc.Country,
				Currency:  loc.Currency,
				Cards:     loc.Cards,
				On:        loc.Active,
			})
		}
	}
	return v, nil
}

func requestCountry(c *zip.Ctx) string { return checkout.RequestCountry(c) }

type routeRequest struct {
	Country     string `json:"country"`
	Currency    string `json:"currency"`
	AmountCents int64  `json:"amountCents"`
	Method      string `json:"method"`
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

func bankAlternatives(ctx context.Context, org *organization.Organization, reg *processor.Registry, pol routing.Policy, country, cur string) []string {
	var out []string
	hasACH := false
	if pol.On(processor.Stripe) {
		if p, err := reg.Registered(processor.Stripe); err == nil && p.IsAvailable(ctx) {
			if c, ok := p.(processor.Capable); ok {
				have := c.Methods(ctx)
				for _, m := range routing.BankMethods(country) {
					if !have[m] || !pol.MethodOn(m) || !routing.MethodViable(m, cur) {
						continue
					}
					if m == "us_bank_account" {
						m = "ach"
						hasACH = true
					}
					out = append(out, m)
				}
			}
		}
	}
	if !hasACH && strings.EqualFold(strings.TrimSpace(country), "US") &&
		pol.On(processor.Square) && pol.CountryProcessor(country) == processor.Square &&
		pol.MethodOn("us_bank_account") && routing.MethodViable("us_bank_account", cur) &&
		!pol.On(processor.Stripe) {
		if sq, err := reg.Registered(processor.Square); err == nil && sq.IsAvailable(ctx) {
			out = append(out, "ach")
		}
	}
	if org != nil && wireConfigured(ctx, org) {
		out = append(out, "wire")
	}
	return out
}

func wireConfigured(ctx context.Context, org *organization.Organization) bool {
	recv := *org
	payment.Hydrate(ctx, &recv, "wire")
	return recv.Wire.AccountNumber != "" || recv.Wire.IBAN != ""
}

func stripeStablecoins(ctx context.Context, reg *processor.Registry) processor.StablecoinReady {
	p, err := reg.Registered(processor.Stripe)
	if err != nil || !p.IsAvailable(ctx) {
		return processor.StablecoinReady{Why: "Stripe is not available"}
	}
	d, ok := p.(processor.StablecoinDepositor)
	if !ok {
		return processor.StablecoinReady{Why: "this Stripe cannot take stablecoin deposits"}
	}
	return d.StablecoinReady(ctx)
}

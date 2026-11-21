package billing

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/billing/engine"
	"github.com/hanzoai/commerce/checkout"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/checkoutsession"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/paymentmethod"
	"github.com/hanzoai/commerce/models/pendingintent"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/payment/processor"
	orgpkg "github.com/hanzoai/commerce/pkg/org"
	types "github.com/hanzoai/commerce/types"
	jsonhttp "github.com/hanzoai/commerce/util/json/http"
)

// This file is the headless purchase surface: a machine-readable USD catalog, a checkout an
// agent creates, updates and completes in JSON, and the order it confirms. Payment arrives as
// a shared payment token the buyer granted the agent (charged through the same processor
// adapter, risk, refund and dispute paths as any card) or as a stablecoin deposit.

// agentVersion names the shape of this surface for clients.
const agentVersion = "1"

// creditPacks are the amounts of USD credit the catalog lists, in cents, within the top-up
// bounds.
var creditPacks = []int64{500, 1000, 2500, 5000, 10000, 25000, 50000}

// agentToken is the payment method type of a stored shared payment token.
const agentToken = "agent_token"

// CreditPack is an amount of USD credit for sale.
type CreditPack struct {
	ID       string `json:"id"`
	UsdCents int64  `json:"usdCents"`
}

// PaymentOption is a way the catalog says a checkout can be paid.
type PaymentOption struct {
	Type      string   `json:"type"`
	Processor string   `json:"processor"`
	Currency  string   `json:"currency"`
	Networks  []string `json:"networks,omitempty"`
	// Recurring says a token of this type may renew a subscription when it carries a
	// recurring interval.
	Recurring bool `json:"recurring,omitempty"`
}

// CatalogOut is the machine-readable USD catalog.
type CatalogOut struct {
	Version     string            `json:"version"`
	Currency    string            `json:"currency"`
	Plans       []PlanView        `json:"plans"`
	CreditPacks []CreditPack      `json:"creditPacks"`
	Payment     []PaymentOption   `json:"payment"`
	Links       map[string]string `json:"links"`
}

// ReadCatalog is what is for sale and how it can be paid for, in dollars.
func ReadCatalog(ctx context.Context, org *organization.Organization, host string) (*CatalogOut, error) {
	plans, err := ReadPlans(ctx, host, "", nil)
	if err != nil {
		return nil, err
	}
	paid := plans[:0:0]
	for _, p := range plans {
		if p.Price > 0 && !p.ContactSales {
			paid = append(paid, p)
		}
	}
	min, max := topupBounds()
	out := &CatalogOut{Version: agentVersion, Currency: "usd", Plans: paid, CreditPacks: []CreditPack{}, Payment: []PaymentOption{}, Links: agentLinks()}
	for _, c := range creditPacks {
		if c >= min && c <= max {
			out.CreditPacks = append(out.CreditPacks, CreditPack{ID: fmt.Sprintf("credit-%d", c), UsdCents: c})
		}
	}
	if org != nil {
		reg := processorsForOrg(org)
		if p, e := reg.Registered(processor.Stripe); e == nil && p.IsAvailable(ctx) {
			if _, ok := p.(processor.GrantReader); ok {
				out.Payment = append(out.Payment, PaymentOption{Type: "shared_payment_token", Processor: "stripe", Currency: "usd", Recurring: true})
			}
		}
		if s := stripeStablecoins(ctx, reg); s.On {
			out.Payment = append(out.Payment, PaymentOption{Type: "stablecoin", Processor: "stripe", Currency: s.Currency, Networks: s.Networks})
		}
	}
	return out, nil
}

func agentLinks() map[string]string {
	return map[string]string{
		"catalog":   "/v1/billing/catalog",
		"discovery": "/v1/billing/discovery",
		"checkouts": "/v1/billing/checkouts",
		"orders":    "/v1/billing/orders",
	}
}

// Discovery tells an agent where the purchase surface is and what it speaks.
type Discovery struct {
	Version  string            `json:"version"`
	Currency string            `json:"currency"`
	Links    map[string]string `json:"links"`
	// Lifecycle is the checkout's verbs, in order.
	Lifecycle []string `json:"lifecycle"`
	// Payment lists the payment types a checkout completes with.
	Payment []string `json:"payment"`
	// Renewal says how a subscription is renewed by an agent.
	Renewal string `json:"renewal"`
}

// GetBillingCatalog serves the catalog.
//
//	GET /v1/billing/catalog
func GetBillingCatalog(c *zip.Ctx) error {
	host := checkout.RequestHost(c)
	org, ok := middleware.GetOrganizationOK(c)
	if !ok || org == nil {
		// Anonymous: the catalog is the brand the host names.
		org, _ = orgpkg.Resolve(c.Context(), checkout.BrandSlugForHost(host))
	}
	out, err := ReadCatalog(c.Context(), org, host)
	if err != nil {
		return jsonhttp.Fail(c, 503, "the catalog is unavailable", err)
	}
	return c.JSON(200, out)
}

// GetBillingDiscovery serves the discovery metadata.
//
//	GET /v1/billing/discovery
func GetBillingDiscovery(c *zip.Ctx) error {
	return c.JSON(200, Discovery{
		Version: agentVersion, Currency: "usd", Links: agentLinks(),
		Lifecycle: []string{"POST /v1/billing/checkouts", "PATCH /v1/billing/checkouts/{id}", "POST /v1/billing/checkouts/{id}/complete", "GET /v1/billing/orders/{id}"},
		Payment:   []string{"shared_payment_token", "stablecoin"},
		Renewal:   "A subscription renews on a shared payment token whose usage limits carry a recurring interval and a maximum at least the plan's renewal amount. A renewal the token cannot cover leaves the subscription past due and announces renewal.action_required with the path to supply a fresh token: POST /v1/billing/invoices/{id}/agent-token.",
	})
}

// ---- crypto top-up: Stripe stablecoins, headless ----

// CryptoTopupIn opens a stablecoin payment for AmountCents of USD credit.
type CryptoTopupIn struct {
	Subject        string
	AmountCents    int64
	Network        string
	IdempotencyKey string
}

// CryptoTopupOut is where to send the stablecoin: per network the address and the tokens it
// takes. Nothing is credited until the deposit is confirmed on chain and Stripe reports the
// payment succeeded, once.
type CryptoTopupOut struct {
	IntentID    string                     `json:"intentId"`
	AmountCents int64                      `json:"amountCents"`
	Currency    string                     `json:"currency"`
	UsdOnly     bool                       `json:"usdOnly"`
	Addresses   []processor.DepositAddress `json:"addresses"`
	Status      string                     `json:"status"`
}

// stripeDeposit opens a Stripe stablecoin deposit of cents USD on a network, after checking
// the account can take stablecoins now. It records nothing: each caller registers the intent
// for what it pays.
func stripeDeposit(ctx context.Context, org *organization.Organization, subject string, cents int64, network, key, desc string) (*processor.StablecoinDeposit, string, *PaymentFault) {
	reg := processorsForOrg(org)
	ready := stripeStablecoins(ctx, reg)
	if !ready.On {
		return nil, "", fault(503, "stablecoin payments are not available right now: "+ready.Why, nil)
	}
	network = strings.ToLower(strings.TrimSpace(network))
	if network == "" {
		network = ready.Networks[0]
	}
	ok := false
	for _, n := range ready.Networks {
		ok = ok || n == network
	}
	if !ok {
		return nil, "", fault(400, "network must be one of "+strings.Join(ready.Networks, ", "), nil)
	}
	sp, err := reg.Existing(ctx, processor.Stripe)
	depositor, isDep := sp.(processor.StablecoinDepositor)
	if err != nil || !isDep {
		return nil, "", fault(503, "Stripe is not available right now", err)
	}
	dep, err := depositor.OpenStablecoinDeposit(ctx, processor.StablecoinDepositIn{
		Amount: cents, Currency: "usd", Network: network,
		Key:      gatewayKey("deposit", subject, fmt.Sprintf("%s:%d", key, cents)),
		Desc:     desc,
		Metadata: map[string]interface{}{"hanzo_subject": subject, "hanzo_usd_cents": cents, "hanzo_org": org.Name, "org": org.Name, "user": strings.ToLower(subject)},
	})
	if err != nil {
		return nil, "", fault(502, "the payment could not be opened", err)
	}
	return dep, network, nil
}

// cancelDeposit cancels a deposit that could not be recorded, so no address is handed out
// whose money nobody could place.
func cancelDeposit(ctx context.Context, org *organization.Organization, id string) {
	if sp, err := processorsForOrg(org).Existing(ctx, processor.Stripe); err == nil {
		if c, ok := sp.(interface {
			CancelIntent(context.Context, string) error
		}); ok {
			_ = c.CancelIntent(ctx, id)
		}
	}
}

// OpenCryptoTopup opens a Stripe stablecoin deposit for a top-up. It is refused unless the
// Stripe account can take stablecoins now.
func OpenCryptoTopup(ctx context.Context, org *organization.Organization, in CryptoTopupIn) (*CryptoTopupOut, *PaymentFault) {
	if org == nil || in.Subject == "" {
		return nil, fault(401, "sign in to add funds", nil)
	}
	min, max := topupBounds()
	if in.AmountCents < min || in.AmountCents > max {
		return nil, fault(400, fmt.Sprintf("amountCents must be between %d and %d", min, max), nil)
	}
	if cerr := atCap(in.Subject); cerr != nil {
		return nil, fault(429, cerr.Error(), cerr)
	}
	key := in.IdempotencyKey
	if key == "" {
		key = windowKey(fmt.Sprintf("crypto:%d:%s", in.AmountCents, in.Network))
	}
	dep, network, f := stripeDeposit(ctx, org, in.Subject, in.AmountCents, in.Network, key, fmt.Sprintf("Top-up %d usd for %s", in.AmountCents, in.Subject))
	if f != nil {
		return nil, f
	}
	_, polVersion, _ := ReadRoutingVersioned(ctx, org.Name)
	if perr := pendingintent.PutQuoted(dep.ID, org.Name, strings.ToLower(in.Subject), pendingintent.Topup, in.AmountCents, "usd", 0, "", polVersion, org.TestMode()); perr != nil {
		cancelDeposit(ctx, org, dep.ID)
		log.Error("RECONCILE: stablecoin intent %s could not be recorded: %v", dep.ID, perr)
		return nil, fault(503, "billing is temporarily unavailable; please retry", perr)
	}
	// A deposit confirms on chain, minutes to hours after the buyer sent it: the attempt waits.
	if berr := pendingintent.Bind(dep.ID, pendingintent.Terms{Delayed: true}); berr != nil {
		log.Warn("stablecoin intent %s: the terms were not bound: %v", dep.ID, berr)
	}
	if _, eerr := engine.EmitBillingEvent(webhookGuardDB(ctx), "topup.crypto.opened", "payment_intent", dep.ID, in.Subject,
		map[string]interface{}{"network": network, "usdCents": in.AmountCents, "org": org.Name, "policyVersion": polVersion}, nil); eerr != nil {
		log.Warn("stablecoin intent %s: the opening was not recorded: %v", dep.ID, eerr)
	}
	return &CryptoTopupOut{IntentID: dep.ID, AmountCents: in.AmountCents, Currency: "usd", UsdOnly: true, Addresses: dep.Addresses, Status: "awaiting_deposit"}, nil
}

type cryptoTopupRequest struct {
	AmountCents int64  `json:"amountCents"`
	Network     string `json:"network"`
}

// CreateCryptoTopup opens a Stripe stablecoin deposit.
//
//	POST /v1/billing/topup/crypto   { amountCents, network? }
func CreateCryptoTopup(c *zip.Ctx) error {
	org, ok := middleware.GetOrganizationOK(c)
	if !ok || org == nil {
		return jsonhttp.Fail(c, 401, "sign in to add funds", nil)
	}
	var req cryptoTopupRequest
	if err := c.Bind(&req); err != nil {
		return jsonhttp.Fail(c, 400, "invalid request body", err)
	}
	out, f := OpenCryptoTopup(c.Context(), org, CryptoTopupIn{
		Subject: userBillingKey(c), AmountCents: req.AmountCents, Network: req.Network,
		IdempotencyKey: strings.TrimSpace(c.Header("X-Idempotency-Key")),
	})
	if f != nil {
		return jsonhttp.Fail(c, f.Status, f.Message, f.Err)
	}
	return c.JSON(200, out)
}

// ---- checkout lifecycle ----

// ItemIn is the one thing a checkout sells: USD credit, or a plan's first period.
type ItemIn struct {
	Kind     string `json:"kind"` // "credit" | "plan"
	UsdCents int64  `json:"usdCents,omitempty"`
	Plan     string `json:"plan,omitempty"`
	Interval string `json:"interval,omitempty"`
}

// CheckoutView is a checkout as the agent reads it.
type CheckoutView struct {
	ID         string     `json:"id"`
	Status     string     `json:"status"`
	Currency   string     `json:"currency"`
	Items      []ItemIn   `json:"items"`
	TotalCents int64      `json:"totalCents"`
	Order      *OrderView `json:"order,omitempty"`
	// Next is what the agent does next when the status is not yet complete.
	Next *NextAction `json:"next,omitempty"`
	// Error is why the last completion was refused, with a code an agent can branch on.
	Error *AgentError       `json:"error,omitempty"`
	Links map[string]string `json:"links"`
}

// OrderView is the confirmation of a completed checkout.
type OrderView struct {
	ID             string `json:"id"`
	CheckoutID     string `json:"checkoutId"`
	Status         string `json:"status"` // "paid"
	Currency       string `json:"currency"`
	TotalCents     int64  `json:"totalCents"`
	CreditedCents  int64  `json:"creditedCents,omitempty"`
	PaymentID      string `json:"paymentId,omitempty"`
	SubscriptionID string `json:"subscriptionId,omitempty"`
	Plan           string `json:"plan,omitempty"`
	// Token is what is left on the shared payment token the subscription renews on.
	Token       *TokenRoom `json:"token,omitempty"`
	CompletedAt string     `json:"completedAt,omitempty"`
}

// NextAction is a step the agent must take for the checkout to go on.
type NextAction struct {
	Type string `json:"type"` // "authenticate" | "send_deposit"
	// authenticate: the bank's step.
	IntentID       string `json:"intentId,omitempty"`
	ClientSecret   string `json:"clientSecret,omitempty"`
	PublishableKey string `json:"publishableKey,omitempty"`
	// send_deposit: where to send the stablecoin.
	Addresses []processor.DepositAddress `json:"addresses,omitempty"`
	Message   string                     `json:"message,omitempty"`
}

// AgentError is a refusal an agent can act on.
type AgentError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func checkoutView(s *checkoutsession.Session) *CheckoutView {
	v := &CheckoutView{ID: s.Id(), Status: s.Status, Currency: "usd", TotalCents: s.TotalCents, Links: map[string]string{
		"self": "/v1/billing/checkouts/" + s.Id(), "complete": "/v1/billing/checkouts/" + s.Id() + "/complete",
	}}
	v.Items = []ItemIn{{Kind: s.Line, UsdCents: s.UsdCents, Plan: s.Plan, Interval: s.Interval}}
	if s.Status == checkoutsession.Complete {
		v.Order = confirmationOf(s)
	}
	if s.Reason != "" && s.Status != checkoutsession.Complete {
		code, msg, _ := strings.Cut(s.Reason, ": ")
		v.Error = &AgentError{Code: code, Message: msg}
	}
	return v
}

func confirmationOf(s *checkoutsession.Session) *OrderView {
	o := &OrderView{ID: s.Id(), CheckoutID: s.Id(), Status: "paid", Currency: "usd", TotalCents: s.TotalCents,
		PaymentID: s.PaymentID, SubscriptionID: s.SubscriptionID, Plan: s.Plan, CompletedAt: s.CompletedAt.UTC().Format(time.RFC3339)}
	if s.Line == checkoutsession.Credit {
		o.CreditedCents = s.UsdCents
	}
	return o
}

// priceLine validates a line and prices it from the catalog, never from the request.
func priceLine(ctx context.Context, org *organization.Organization, host string, l ItemIn) (total int64, fail *PaymentFault) {
	switch strings.ToLower(l.Kind) {
	case checkoutsession.Credit:
		min, max := topupBounds()
		if l.UsdCents < min || l.UsdCents > max {
			return 0, fault(400, fmt.Sprintf("usdCents must be between %d and %d", min, max), nil)
		}
		return l.UsdCents, nil
	case checkoutsession.Plan:
		plans, err := ReadPlans(ctx, host, "", nil)
		if err != nil {
			return 0, fault(503, "the catalog is unavailable", err)
		}
		for _, p := range plans {
			if p.Slug != l.Plan {
				continue
			}
			if p.Price <= 0 || p.ContactSales {
				return 0, fault(422, "this plan is not sold here", nil)
			}
			if l.Interval != "" && l.Interval != "month" {
				return 0, fault(422, "agents buy the monthly price", nil)
			}
			return p.Price, nil
		}
		return 0, fault(404, "no such plan", nil)
	}
	return 0, fault(400, "kind must be credit or plan", nil)
}

type checkoutRequest struct {
	Items []ItemIn `json:"items"`
}

func newSessionID() string {
	var b [12]byte
	_, _ = rand.Read(b[:])
	return "chk_" + hex.EncodeToString(b[:])
}

// CreateCheckout opens a checkout for one line.
func CreateCheckout(ctx context.Context, org *organization.Organization, subject, host string, items []ItemIn) (*CheckoutView, *PaymentFault) {
	if org == nil || subject == "" {
		return nil, fault(401, "sign in to check out", nil)
	}
	if len(items) != 1 {
		return nil, fault(400, "a checkout sells exactly one item", nil)
	}
	total, f := priceLine(ctx, org, host, items[0])
	if f != nil {
		return nil, f
	}
	s := new(checkoutsession.Session)
	s.SetId(newSessionID())
	s.Org, s.Subject, s.Test, s.Status = org.Name, strings.ToLower(subject), org.TestMode(), checkoutsession.Open
	s.Line, s.UsdCents, s.Plan, s.Interval, s.TotalCents = strings.ToLower(items[0].Kind), items[0].UsdCents, items[0].Plan, items[0].Interval, total
	if s.Line == checkoutsession.Plan {
		s.UsdCents = 0
	}
	if err := checkoutsession.Put(s); err != nil {
		return nil, fault(503, "billing is temporarily unavailable; please retry", err)
	}
	return checkoutView(s), nil
}

func loadSession(org *organization.Organization, subject, id string) (*checkoutsession.Session, *PaymentFault) {
	s, found, err := checkoutsession.Get(id)
	if err != nil {
		return nil, fault(503, "billing is temporarily unavailable; please retry", err)
	}
	if !found || !strings.EqualFold(s.Subject, subject) || !strings.EqualFold(s.Org, org.Name) {
		return nil, fault(404, "checkout not found", nil)
	}
	return s, nil
}

// UpdateCheckout replaces the item of a checkout that is still open.
func UpdateCheckout(ctx context.Context, org *organization.Organization, subject, host, id string, items []ItemIn) (*CheckoutView, *PaymentFault) {
	if len(items) != 1 {
		return nil, fault(400, "a checkout sells exactly one item", nil)
	}
	s, f := loadSession(org, subject, id)
	if f != nil {
		return nil, f
	}
	total, f := priceLine(ctx, org, host, items[0])
	if f != nil {
		return nil, f
	}
	got, changed, err := checkoutsession.Change(id, func(s *checkoutsession.Session) bool {
		if s.Status != checkoutsession.Open && s.Status != checkoutsession.Failed {
			return false
		}
		s.Status, s.Reason = checkoutsession.Open, ""
		s.Line, s.UsdCents, s.Plan, s.Interval, s.TotalCents = strings.ToLower(items[0].Kind), items[0].UsdCents, items[0].Plan, items[0].Interval, total
		if s.Line == checkoutsession.Plan {
			s.UsdCents = 0
		}
		return true
	})
	if err != nil {
		return nil, fault(503, "billing is temporarily unavailable; please retry", err)
	}
	if !changed {
		return nil, fault(409, "this checkout is "+s.Status+" and can no longer be changed", nil)
	}
	return checkoutView(got), nil
}

// ReadCheckout answers where a checkout is; one waiting on a deposit is settled from the
// credit when the deposit has been credited.
func ReadCheckout(ctx context.Context, org *organization.Organization, subject, id string) (*CheckoutView, *PaymentFault) {
	s, f := loadSession(org, subject, id)
	if f != nil {
		return nil, f
	}
	if s.Status == checkoutsession.Pending && s.Intent != "" {
		if ref, credited := creditedBy(ctx, org, s.Intent); credited {
			got, _, err := checkoutsession.Change(id, func(s *checkoutsession.Session) bool {
				if s.Status == checkoutsession.Complete {
					return false
				}
				s.Status, s.PaymentID, s.CompletedAt = checkoutsession.Complete, ref, time.Now()
				return true
			})
			if err == nil && got != nil {
				s = got
			}
		}
	}
	v := checkoutView(s)
	if v.Order != nil && v.Order.SubscriptionID != "" {
		v.Order.Token = subscriptionRoom(ctx, org, v.Order.SubscriptionID)
	}
	if s.Status == checkoutsession.Pending && s.Payment == "stablecoin" {
		v.Next = &NextAction{Type: "send_deposit", IntentID: s.Intent, Message: "waiting for the deposit to confirm on chain"}
	}
	return v, nil
}

// creditedBy says the payment intent's credit has been posted, and names the receipt.
func creditedBy(ctx context.Context, org *organization.Organization, intent string) (string, bool) {
	st, f := ReadTopupIntent(ctx, org, subjectOfIntent(intent), intent)
	if f != nil || st == nil || !st.Credited {
		return "", false
	}
	return st.PaymentID, true
}

func subjectOfIntent(intent string) string {
	p, found, _ := pendingintent.Get(intent)
	if !found {
		return ""
	}
	return p.Subject
}

// ReadOrder is the confirmation of a completed checkout.
func ReadOrder(ctx context.Context, org *organization.Organization, subject, id string) (*OrderView, *PaymentFault) {
	v, f := ReadCheckout(ctx, org, subject, id)
	if f != nil {
		return nil, f
	}
	if v.Order == nil {
		return nil, fault(404, "this checkout has no order yet", nil)
	}
	return v.Order, nil
}

// ---- completion ----

// PaymentIn is how the agent pays: a shared payment token, or a stablecoin deposit.
type PaymentIn struct {
	Type    string `json:"type"` // "shared_payment_token" | "stablecoin"
	Token   string `json:"token,omitempty"`
	Network string `json:"network,omitempty"`
}

// CompleteOut is the result of a completion: the checkout, and the HTTP status to answer.
type CompleteOut struct {
	Status int
	View   *CheckoutView
}

// CompleteCheckout pays a checkout. The charge is bound to the checkout, the buyer and the
// idempotency key it arrived with, so completing twice is one purchase.
func CompleteCheckout(ctx context.Context, org *organization.Organization, subject, host, id, key string, pay PaymentIn) CompleteOut {
	fail := func(status int, s *checkoutsession.Session, code, msg string) CompleteOut {
		if s != nil {
			_, _, _ = checkoutsession.Change(s.Id(), func(x *checkoutsession.Session) bool {
				if x.Status == checkoutsession.Complete || x.Status == checkoutsession.Pending {
					return false
				}
				x.Status, x.Reason = checkoutsession.Failed, code+": "+msg
				return true
			})
			s.Status, s.Reason = checkoutsession.Failed, code+": "+msg
		}
		v := &CheckoutView{ID: id, Status: checkoutsession.Failed, Currency: "usd", Error: &AgentError{Code: code, Message: msg}, Links: map[string]string{}}
		if s != nil {
			v = checkoutView(s)
			v.Error = &AgentError{Code: code, Message: msg}
		}
		return CompleteOut{Status: status, View: v}
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return fail(400, nil, "idempotency_key_required", "send an Idempotency-Key header: it binds the charge to this checkout")
	}
	s, f := loadSession(org, subject, id)
	if f != nil {
		return fail(f.Status, nil, "not_found", f.Message)
	}
	// A completed checkout answers the same order to the same key and refuses another.
	if s.Status == checkoutsession.Complete || s.Status == checkoutsession.Pending {
		if s.Key == key {
			v, _ := ReadCheckout(ctx, org, subject, id)
			if v != nil && v.Status == checkoutsession.Complete {
				return CompleteOut{Status: 200, View: v}
			}
			if v != nil {
				return CompleteOut{Status: 202, View: v}
			}
		}
		return fail(409, s, "already_completed", "this checkout was completed under another idempotency key")
	}

	// Take the checkout: one completion at a time wins it.
	taken, won, err := checkoutsession.Change(id, func(x *checkoutsession.Session) bool {
		if x.Status != checkoutsession.Open && x.Status != checkoutsession.Failed {
			return false
		}
		x.Status, x.Key, x.Payment, x.Reason = checkoutsession.Pending, key, strings.ToLower(pay.Type), ""
		return true
	})
	if err != nil {
		return fail(503, nil, "unavailable", "billing is temporarily unavailable; please retry")
	}
	if !won {
		return fail(409, s, "in_progress", "this checkout is being completed")
	}
	s = taken
	release := func(status int, code, msg string) CompleteOut {
		_, _, _ = checkoutsession.Change(id, func(x *checkoutsession.Session) bool {
			x.Status, x.Reason = checkoutsession.Failed, code+": "+msg
			return true
		})
		s.Status, s.Reason = checkoutsession.Failed, code+": "+msg
		v := checkoutView(s)
		v.Error = &AgentError{Code: code, Message: msg}
		return CompleteOut{Status: status, View: v}
	}

	switch strings.ToLower(pay.Type) {
	case "stablecoin":
		if s.Line == checkoutsession.Plan {
			return completePlanByDeposit(ctx, org, subject, s, key, pay.Network, release)
		}
		out, f := OpenCryptoTopup(ctx, org, CryptoTopupIn{Subject: subject, AmountCents: s.UsdCents, Network: pay.Network, IdempotencyKey: "checkout:" + id + ":" + key})
		if f != nil {
			return release(f.Status, "stablecoin_unavailable", f.Message)
		}
		got, _, _ := checkoutsession.Change(id, func(x *checkoutsession.Session) bool { x.Intent = out.IntentID; return true })
		if got != nil {
			s = got
		}
		v := checkoutView(s)
		v.Next = &NextAction{Type: "send_deposit", IntentID: out.IntentID, Addresses: out.Addresses, Message: "send USD stablecoin to an address; the checkout completes when the deposit confirms"}
		return CompleteOut{Status: 202, View: v}

	case "shared_payment_token":
		return completeWithToken(ctx, org, subject, s, key, pay.Token, release)
	}
	return release(400, "payment_type_unknown", "payment.type must be shared_payment_token or stablecoin")
}

// readGrant retrieves the token the buyer granted and validates it for a charge of cents:
// it must be this seller's, in this mode, live, in dollars, unexpired and within its limit.
// recurring additionally requires an interval, for a token that will renew a subscription.
func readGrant(ctx context.Context, org *organization.Organization, token string, cents int64, recurring bool) (*processor.Grant, *AgentError) {
	token = strings.TrimSpace(token)
	if !strings.HasPrefix(token, "spt_") {
		return nil, &AgentError{"token_invalid", "payment.token must be a shared payment token (spt_...)"}
	}
	sp, err := processorsForOrg(org).Existing(context.WithoutCancel(ctx), processor.Stripe)
	gr, ok := sp.(processor.GrantReader)
	if err != nil || !ok {
		return nil, &AgentError{"payments_unavailable", "shared payment tokens cannot be accepted right now"}
	}
	g, err := gr.Grant(ctx, token)
	if err != nil {
		// Stripe answers a token that is not granted to this seller as not found.
		return nil, &AgentError{"token_wrong_merchant", "this token was not granted to this seller"}
	}
	if g.Test != org.TestMode() {
		return nil, &AgentError{"token_wrong_mode", "this token is for the other mode (test or live)"}
	}
	if code, why := processor.GrantRefusal(g, cents, "usd", recurring, time.Now()); why != "" {
		return g, &AgentError{code, why}
	}
	return g, nil
}

func completeWithToken(ctx context.Context, org *organization.Organization, subject string, s *checkoutsession.Session, key, token string,
	release func(int, string, string) CompleteOut) CompleteOut {
	id := s.Id()
	plan := s.Line == checkoutsession.Plan
	g, aerr := readGrant(ctx, org, token, s.TotalCents, plan)
	if aerr != nil && !(plan && aerr.Code == "token_not_recurring") {
		return release(402, aerr.Code, aerr.Message)
	}
	// A plan paid with a token that is not recurring is bought for its first period only: the
	// token is not kept, and the subscription announces the renewal action when it falls due.
	recurring := g != nil && g.RecurringInterval != ""
	bound := "checkout:" + id + ":" + key
	done := func(paymentID, subscriptionID string) CompleteOut {
		got, _, _ := checkoutsession.Change(id, func(x *checkoutsession.Session) bool {
			x.Status, x.PaymentID, x.SubscriptionID, x.CompletedAt, x.Reason = checkoutsession.Complete, paymentID, subscriptionID, time.Now(), ""
			return true
		})
		if got != nil {
			s = got
		}
		return CompleteOut{Status: 200, View: checkoutView(s)}
	}

	if !plan {
		out, f := TakePayment(ctx, org, TakePaymentIn{SourceID: token, AmountCents: s.UsdCents, Currency: "usd", Subject: subject, Provider: "stripe", IdempotencyKey: bound})
		if f != nil {
			return refusedPayment(release, f)
		}
		return done(out.TransactionID, "")
	}

	// A subscription: the token becomes the subscription's payment method, which the first
	// period and every renewal charge; a token without a recurring interval is dropped after
	// the first period.
	db := datastore.New(org.Namespaced(ctx))
	pm, err := tokenMethod(db, subject, token, g)
	if err != nil {
		return release(503, "unavailable", "billing is temporarily unavailable; please retry")
	}
	sale, err := Subscribe(ctx, org, SubscribeIn{
		MethodID: pm.Id(), PlanID: s.Plan, Subject: subject, Provider: string(processor.Stripe),
		IdempotencyKey: bound, Interval: types.Interval(""),
		// The first charge is the total the checkout quoted, or no charge.
		ExpectedCents: s.TotalCents,
	})
	if err != nil {
		_ = pm.Delete()
		return releaseSale(release, err)
	}
	if !recurring {
		dropToken(db, sale.SubscriptionID, pm)
	}
	return done(sale.InvoiceID, sale.SubscriptionID)
}

// tokenMethod stores a shared payment token as a payment method of its subject, with the
// limits it was read with.
func tokenMethod(db *datastore.Datastore, subject, token string, g *processor.Grant) (*paymentmethod.PaymentMethod, error) {
	pm := paymentmethod.New(db)
	pm.CustomerId, pm.UserId = subject, subject
	pm.Type, pm.ProviderRef, pm.ProviderType = agentToken, token, string(processor.Stripe)
	pm.Name = "Agent payment token"
	pm.Metadata = map[string]interface{}{"customerId": subject, "recurringInterval": g.RecurringInterval, "maxAmount": g.MaxAmount, "expiresAt": g.ExpiresAt}
	if g.Last4 != "" {
		pm.Card = &paymentmethod.CardDetails{Brand: g.Brand, Last4: g.Last4, Country: g.Country}
	}
	return pm, pm.Create()
}

// dropToken removes a one-time token from a subscription it paid the first period of, so it
// can never be offered for a renewal.
func dropToken(db *datastore.Datastore, subscriptionID string, pm *paymentmethod.PaymentMethod) {
	sub := subscription.New(db)
	if err := sub.GetById(subscriptionID); err == nil && sub.DefaultPaymentMethod == pm.Id() {
		sub.DefaultPaymentMethod = ""
		if err := sub.Update(); err != nil {
			log.Error("RECONCILE: subscription %s still holds a one-time token: %v", subscriptionID, err)
		}
	}
	if err := pm.Delete(); err != nil {
		log.Warn("a one-time token's payment method %s was not removed: %v", pm.Id(), err)
	}
}

type authRequiredBody struct {
	Code         string `json:"code"`
	ID           string `json:"id"`
	ClientSecret string `json:"clientSecret"`
	Message      string `json:"message"`
}

// refusedPayment maps a payment fault to a typed refusal: a bank's authentication step is
// authentication_required with what to do next, never a fake success.
func refusedPayment(release func(int, string, string) CompleteOut, f *PaymentFault) CompleteOut {
	var ar authRequiredBody
	if json.Unmarshal([]byte(f.Message), &ar) == nil && ar.Code == "authentication_required" {
		out := release(402, "authentication_required", firstNonEmpty(ar.Message, "the card's bank requires authentication"))
		out.View.Next = &NextAction{Type: "authenticate", IntentID: ar.ID, ClientSecret: ar.ClientSecret}
		return out
	}
	var pe *processor.PaymentError
	if errors.As(f.Err, &pe) && strings.HasPrefix(pe.Code, "token_") {
		return release(402, pe.Code, pe.Message)
	}
	switch f.Status {
	case 402:
		return release(402, "declined", f.Message)
	case 409:
		return release(409, "in_progress", f.Message)
	}
	return release(f.Status, "payment_failed", f.Message)
}

func releaseSale(release func(int, string, string) CompleteOut, err error) CompleteOut {
	var moved *Refusal
	if errors.As(err, &moved) && moved.Code == "price_changed" {
		// The catalog moved after the quote: nothing was charged, and updating the checkout
		// quotes the price now.
		return release(409, "price_changed", err.Error())
	}
	var pe *processor.PaymentError
	if errors.As(err, &pe) && strings.HasPrefix(pe.Code, "token_") {
		return release(402, pe.Code, pe.Message)
	}
	switch {
	case IsSaleDeclined(err):
		return release(402, "declined", err.Error())
	case IsSaleConflict(err):
		return release(409, "in_progress", err.Error())
	case IsSaleNotFound(err):
		return release(404, "not_found", err.Error())
	}
	return release(422, "sale_refused", err.Error())
}

// ---- renewal: the action that lets an agent keep a subscription ----

// announceTokenRenewal tells an agent a renewal on its token could not be taken, with a code
// to branch on and the path to supply a fresh token that covers it.
func announceTokenRenewal(ctx context.Context, sub *subscription.Subscription, inv invoiceRef, code, why string) {
	if _, err := engine.EmitBillingEvent(webhookGuardDB(ctx), "renewal.action_required", "subscription", sub.Id(), sub.UserId,
		map[string]interface{}{"invoiceId": inv.ID, "subscriptionId": sub.Id(), "code": code, "reason": why,
			"action": map[string]interface{}{"type": "supply_token", "method": "POST", "path": "/v1/billing/invoices/" + inv.ID + "/agent-token",
				"requires": map[string]interface{}{"recurringInterval": "month", "minMaxAmountCents": inv.Cents}}}, nil); err != nil {
		log.Error("renewal %s: the action required was not announced: %v", inv.ID, err)
	}
}

// invoiceRef is the invoice a renewal event names.
type invoiceRef struct {
	ID    string
	Cents int64
}

// tokenCode is the typed code of a charge refused for its token, "" for any other failure.
func tokenCode(res *processor.PaymentResult, err error) (code, why string) {
	var pe *processor.PaymentError
	if errors.As(err, &pe) && strings.HasPrefix(pe.Code, "token_") {
		return pe.Code, pe.Message
	}
	if res != nil && res.Error != nil && errors.As(res.Error, &pe) && strings.HasPrefix(pe.Code, "token_") {
		return pe.Code, pe.Message
	}
	return "", ""
}

type agentTokenRequest struct {
	Token string `json:"token"`
}

// PayInvoiceWithToken supplies a fresh shared payment token for a past-due invoice: it
// becomes the subscription's payment method when it carries a recurring interval covering
// the renewal, and the invoice is charged on it now.
func PayInvoiceWithToken(ctx context.Context, org *organization.Organization, subject, invoiceID, token string) (*AgentPayment, *PaymentFault) {
	inv, db, f := loadInvoice(ctx, org, invoiceID)
	if f != nil {
		return nil, f
	}
	if !strings.EqualFold(inv.UserId, subject) {
		return nil, fault(404, "invoice not found", nil)
	}
	owed := inv.AmountDue
	g, aerr := readGrant(ctx, org, token, owed, true)
	if aerr != nil {
		return &AgentPayment{Status: "declined", Error: aerr}, nil
	}
	if inv.SubscriptionId == "" {
		return nil, fault(422, "this invoice has no subscription to renew", nil)
	}
	pm, err := tokenMethod(db, subject, token, g)
	if err != nil {
		return nil, fault(503, "billing is temporarily unavailable; please retry", err)
	}
	sub := subscription.New(db)
	if err := sub.GetById(inv.SubscriptionId); err != nil {
		_ = pm.Delete()
		return nil, fault(404, "subscription not found", err)
	}
	prev := sub.DefaultPaymentMethod
	sub.DefaultPaymentMethod = pm.Id()
	if err := sub.Update(); err != nil {
		_ = pm.Delete()
		return nil, fault(503, "billing is temporarily unavailable; please retry", err)
	}
	pay, pf := PayInvoiceNow(ctx, org, invoiceID, nil)
	if pf != nil {
		return nil, pf
	}
	if pay.Status != PayPaid {
		// The token did not pay: the subscription keeps the method it had.
		sub.DefaultPaymentMethod = prev
		_ = sub.Update()
		_ = pm.Delete()
		code, msg := "declined", pay.Reason
		if pay.Status == PayAuthentication {
			code, msg = "authentication_required", "the card's bank requires authentication"
		}
		return &AgentPayment{Status: pay.Status, Error: &AgentError{code, msg}, Invoice: pay.Invoice}, nil
	}
	return &AgentPayment{Status: PayPaid, Invoice: pay.Invoice}, nil
}

// AgentPayment is the result of paying an invoice with a token.
type AgentPayment struct {
	Status  string       `json:"status"`
	Error   *AgentError  `json:"error,omitempty"`
	Invoice *InvoiceView `json:"invoice,omitempty"`
}

// ---- handlers ----

func agentCaller(c *zip.Ctx) (*organization.Organization, string, bool) {
	org, ok := middleware.GetOrganizationOK(c)
	if !ok || org == nil {
		return nil, "", false
	}
	return org, userBillingKey(c), true
}

// CreateAgentCheckout opens a checkout.
//
//	POST /v1/billing/checkouts   { items: [{ kind, usdCents | plan }] }
func CreateAgentCheckout(c *zip.Ctx) error {
	org, subject, ok := agentCaller(c)
	if !ok {
		return jsonhttp.Fail(c, 401, "sign in to check out", nil)
	}
	var req checkoutRequest
	if err := c.Bind(&req); err != nil {
		return jsonhttp.Fail(c, 400, "invalid request body", err)
	}
	v, f := CreateCheckout(c.Context(), org, subject, checkout.RequestHost(c), req.Items)
	if f != nil {
		return jsonhttp.Fail(c, f.Status, f.Message, f.Err)
	}
	return c.JSON(http.StatusCreated, v)
}

// GetAgentCheckout reads a checkout.
//
//	GET /v1/billing/checkouts/:id
func GetAgentCheckout(c *zip.Ctx) error {
	org, subject, ok := agentCaller(c)
	if !ok {
		return jsonhttp.Fail(c, 401, "sign in to check out", nil)
	}
	v, f := ReadCheckout(c.Context(), org, subject, c.Param("id"))
	if f != nil {
		return jsonhttp.Fail(c, f.Status, f.Message, f.Err)
	}
	return c.JSON(200, v)
}

// UpdateAgentCheckout changes the item of an open checkout.
//
//	PATCH /v1/billing/checkouts/:id   { items: [...] }
func UpdateAgentCheckout(c *zip.Ctx) error {
	org, subject, ok := agentCaller(c)
	if !ok {
		return jsonhttp.Fail(c, 401, "sign in to check out", nil)
	}
	var req checkoutRequest
	if err := c.Bind(&req); err != nil {
		return jsonhttp.Fail(c, 400, "invalid request body", err)
	}
	v, f := UpdateCheckout(c.Context(), org, subject, checkout.RequestHost(c), c.Param("id"), req.Items)
	if f != nil {
		return jsonhttp.Fail(c, f.Status, f.Message, f.Err)
	}
	return c.JSON(200, v)
}

type completeRequest struct {
	Payment PaymentIn `json:"payment"`
}

// CompleteAgentCheckout pays a checkout.
//
//	POST /v1/billing/checkouts/:id/complete   { payment: { type, token | network } }   Idempotency-Key
func CompleteAgentCheckout(c *zip.Ctx) error {
	org, subject, ok := agentCaller(c)
	if !ok {
		return jsonhttp.Fail(c, 401, "sign in to check out", nil)
	}
	var req completeRequest
	if err := c.Bind(&req); err != nil {
		return jsonhttp.Fail(c, 400, "invalid request body", err)
	}
	key := firstNonEmpty(c.Header("Idempotency-Key"), c.Header("X-Idempotency-Key"))
	out := CompleteCheckout(c.Context(), org, subject, checkout.RequestHost(c), c.Param("id"), key, req.Payment)
	return c.JSON(out.Status, out.View)
}

// GetAgentOrder confirms a completed checkout.
//
//	GET /v1/billing/orders/:id
func GetAgentOrder(c *zip.Ctx) error {
	org, subject, ok := agentCaller(c)
	if !ok {
		return jsonhttp.Fail(c, 401, "sign in to check out", nil)
	}
	o, f := ReadOrder(c.Context(), org, subject, c.Param("id"))
	if f != nil {
		return jsonhttp.Fail(c, f.Status, f.Message, f.Err)
	}
	return c.JSON(200, o)
}

// SupplyAgentTokenAhead replaces the token a subscription renews on before a renewal is due.
//
//	POST /v1/billing/subscriptions/:id/agent-token   { token }
func SupplyAgentTokenAhead(c *zip.Ctx) error {
	org, subject, ok := agentCaller(c)
	if !ok {
		return jsonhttp.Fail(c, 401, "sign in to check out", nil)
	}
	var req agentTokenRequest
	if err := c.Bind(&req); err != nil {
		return jsonhttp.Fail(c, 400, "invalid request body", err)
	}
	room, aerr, f := SupplySubscriptionToken(c.Context(), org, subject, c.Param("id"), req.Token)
	switch {
	case f != nil:
		return jsonhttp.Fail(c, f.Status, f.Message, f.Err)
	case aerr != nil:
		return c.JSON(402, map[string]any{"status": "declined", "error": aerr})
	}
	return c.JSON(200, map[string]any{"status": "ok", "token": room})
}

// SupplyAgentToken pays a past-due renewal with a fresh token.
//
//	POST /v1/billing/invoices/:id/agent-token   { token }
func SupplyAgentToken(c *zip.Ctx) error {
	org, subject, ok := agentCaller(c)
	if !ok {
		return jsonhttp.Fail(c, 401, "sign in to check out", nil)
	}
	var req agentTokenRequest
	if err := c.Bind(&req); err != nil {
		return jsonhttp.Fail(c, 400, "invalid request body", err)
	}
	out, f := PayInvoiceWithToken(c.Context(), org, subject, c.Param("id"), req.Token)
	if f != nil {
		return jsonhttp.Fail(c, f.Status, f.Message, f.Err)
	}
	status := 200
	if out.Status != PayPaid {
		status = 402
	}
	return c.JSON(status, out)
}

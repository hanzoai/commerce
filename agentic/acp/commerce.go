// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package acp

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/hanzoai/commerce/api/billing"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/checkoutsession"
	"github.com/hanzoai/commerce/models/idempotencykey"
	"github.com/hanzoai/commerce/models/organization"
	orgpkg "github.com/hanzoai/commerce/pkg/org"
	"github.com/hanzoai/commerce/util/nscontext"
)

// Commerce is the Backend over commerce's headless purchase surface (api/billing: the
// catalog, CreateCheckout, UpdateCheckout, ReadCheckout and CompleteCheckout). It does no
// pricing and moves no money of its own: an item id is a catalog id (a credit pack,
// credit-<cents>, or a plan slug), the price is the catalog's, and the payment is the
// Stripe shared payment token the buyer granted the agent, charged by CompleteCheckout.
//
// Commerce sells exactly one unit of one item per checkout, in USD, as digital goods. It
// refuses what that cannot honour: a second item, another currency, a payment that is not
// a Stripe shared payment token.
type Commerce struct{}

// Currency implements Backend: the catalog is in USD.
func (Commerce) Currency() string { return "usd" }

// Open implements Backend.
func (c Commerce) Open(ctx context.Context, who Caller, item string) (*Checkout, error) {
	org, err := c.org(ctx, who)
	if err != nil {
		return nil, err
	}
	line, p, err := sold(ctx, org, who.Host, item)
	if err != nil {
		return nil, err
	}
	v, f := billing.CreateCheckout(ctx, org, who.Subject, who.Host, []billing.ItemIn{line})
	if f != nil {
		return nil, refused(f)
	}
	return checkout(v, p), nil
}

// Change implements Backend.
func (c Commerce) Change(ctx context.Context, who Caller, id, item string) (*Checkout, error) {
	org, err := c.org(ctx, who)
	if err != nil {
		return nil, err
	}
	line, p, err := sold(ctx, org, who.Host, item)
	if err != nil {
		return nil, err
	}
	v, f := billing.UpdateCheckout(ctx, org, who.Subject, who.Host, id, []billing.ItemIn{line})
	if f != nil {
		return nil, refused(f)
	}
	return checkout(v, p), nil
}

// Read implements Backend.
func (c Commerce) Read(ctx context.Context, who Caller, id string) (*Checkout, error) {
	org, err := c.org(ctx, who)
	if err != nil {
		return nil, err
	}
	v, f := billing.ReadCheckout(ctx, org, who.Subject, id)
	if f != nil {
		return nil, refused(f)
	}
	return checkout(v, named(ctx, who.Host, v.Items[0])), nil
}

// Pay implements Backend. The charge is bound to the checkout and the key, so a completion
// sent twice is one purchase.
func (c Commerce) Pay(ctx context.Context, who Caller, id, key string, pay Charge) (*Checkout, error) {
	switch {
	case pay.Handler.PSP != "stripe":
		return nil, Unsupported("$.payment_data.handler_id", "Only Stripe payment handlers are accepted.")
	case pay.Instrument != "card":
		return nil, Unsupported("$.payment_data.instrument.type", "Only card instruments are accepted.")
	case pay.Credential != "spt":
		return nil, Unsupported("$.payment_data.instrument.credential.type", "Only shared payment tokens (spt) are accepted.")
	case !strings.HasPrefix(pay.Token, "spt_"):
		return nil, Unsupported("$.payment_data.instrument.credential.token", "Only Stripe shared payment tokens (spt_...) are accepted.")
	}
	org, err := c.org(ctx, who)
	if err != nil {
		return nil, err
	}
	// A canceled checkout is refused here: completion would take it for one in progress.
	v, f := billing.ReadCheckout(ctx, org, who.Subject, id)
	if f != nil {
		return nil, refused(f)
	}
	if v.Status == checkoutsession.Canceled {
		return nil, Conflict("The checkout session is canceled.")
	}
	out := billing.CompleteCheckout(ctx, org, who.Subject, who.Host, id, key, billing.PaymentIn{Type: "shared_payment_token", Token: pay.Token})
	return outcome(out, named(ctx, who.Host, v.Items[0]))
}

// Cancel implements Backend. Commerce keeps a canceled status for a session but exposes no
// call to reach it, so it is set on the model, only on a session that is still open.
func (c Commerce) Cancel(ctx context.Context, who Caller, id string) (*Checkout, error) {
	org, err := c.org(ctx, who)
	if err != nil {
		return nil, err
	}
	v, f := billing.ReadCheckout(ctx, org, who.Subject, id)
	if f != nil {
		return nil, refused(f)
	}
	got, changed, err := checkoutsession.Change(id, func(s *checkoutsession.Session) bool {
		if s.Status != checkoutsession.Open && s.Status != checkoutsession.Failed {
			return false
		}
		s.Status, s.Reason = checkoutsession.Canceled, ""
		return true
	})
	switch {
	case err != nil:
		return nil, Unavailable("Billing is temporarily unavailable; please retry.")
	case !changed && got != nil:
		return nil, NotCancelable("The checkout session is " + got.Status + " and cannot be canceled.")
	case !changed:
		return nil, Unavailable("Billing is temporarily unavailable; please retry.")
	}
	v.Status = checkoutsession.Canceled
	return checkout(v, named(ctx, who.Host, v.Items[0])), nil
}

func (Commerce) org(ctx context.Context, who Caller) (*organization.Organization, error) {
	org, err := orgpkg.Resolve(ctx, who.Org)
	switch {
	case errors.Is(err, orgpkg.ErrResolveFailed):
		return nil, Unavailable("Billing is temporarily unavailable; please retry.")
	case err != nil:
		return nil, err
	}
	return org, nil
}

// sold resolves an item id to what commerce sells: a credit pack of the catalog by its id,
// or a plan by its slug. The price is read back from the catalog by CreateCheckout.
func sold(ctx context.Context, org *organization.Organization, host, item string) (billing.ItemIn, Product, error) {
	cat, err := billing.ReadCatalog(ctx, org, host)
	if err != nil {
		return billing.ItemIn{}, Product{}, Unavailable("The catalog is unavailable.")
	}
	for _, p := range cat.CreditPacks {
		if p.ID == item {
			return billing.ItemIn{Kind: checkoutsession.Credit, UsdCents: p.UsdCents}, pack(p.UsdCents), nil
		}
	}
	for _, p := range cat.Plans {
		if p.Slug == item {
			return billing.ItemIn{Kind: checkoutsession.Plan, Plan: p.Slug}, Product{ID: p.Slug, Name: p.Name}, nil
		}
	}
	return billing.ItemIn{}, Product{}, ErrNoItem
}

func pack(cents int64) Product {
	return Product{ID: fmt.Sprintf("credit-%d", cents), Name: fmt.Sprintf("$%d.%02d USD credit", cents/100, cents%100)}
}

// named is the product a checkout line stands for: a credit pack by its cents, a plan by
// its slug with the name the catalog gives it.
func named(ctx context.Context, host string, l billing.ItemIn) Product {
	if l.Kind != checkoutsession.Plan {
		return pack(l.UsdCents)
	}
	plans, err := billing.ReadPlans(ctx, host, "", nil)
	if err == nil {
		for _, p := range plans {
			if p.Slug == l.Plan {
				return Product{ID: p.Slug, Name: p.Name}
			}
		}
	}
	return Product{ID: l.Plan, Name: l.Plan}
}

// checkout states a commerce checkout view as a Checkout.
func checkout(v *billing.CheckoutView, p Product) *Checkout {
	c := &Checkout{ID: v.ID, Item: p, Total: v.TotalCents}
	switch v.Status {
	case checkoutsession.Complete:
		c.State = Paid
	case checkoutsession.Pending:
		c.State = Paying
	case checkoutsession.Canceled:
		c.State = Canceled
	case checkoutsession.Failed:
		c.State = Failed
	default:
		c.State = Open
	}
	if v.Order != nil {
		c.Receipt = &Receipt{ID: v.Order.ID}
	}
	if c.State == Failed && v.Error != nil {
		c.Refusal = decline(v.Error)
	}
	return c
}

// outcome maps what CompleteCheckout answered. A paid checkout is complete; a payment the
// bank or the token refused leaves the checkout usable and is stated as a refusal; the rest
// are faults of the request or of the seller's systems.
func outcome(out billing.CompleteOut, p Product) (*Checkout, error) {
	v := out.View
	if v == nil {
		return nil, errors.New("acp: CompleteCheckout answered no checkout")
	}
	if out.Status >= http.StatusInternalServerError {
		msg := "Billing is temporarily unavailable; please retry."
		if v.Error != nil {
			msg = v.Error.Message
		}
		return nil, Unavailable(msg)
	}
	if v.Error == nil {
		if v.Status == checkoutsession.Complete || v.Status == checkoutsession.Pending {
			return checkout(v, p), nil
		}
		return nil, fmt.Errorf("acp: CompleteCheckout answered %d with the checkout %s", out.Status, v.Status)
	}
	switch v.Error.Code {
	case "not_found":
		return nil, NotFound(v.Error.Message)
	case "already_completed", "in_progress":
		return nil, Conflict(v.Error.Message)
	case "unavailable", "stablecoin_unavailable":
		return nil, Unavailable(v.Error.Message)
	case "idempotency_key_required", "payment_type_unknown":
		return nil, fmt.Errorf("acp: CompleteCheckout refused the request it was given: %s", v.Error.Code)
	}
	c := checkout(v, p)
	c.State = Failed
	c.Refusal = decline(v.Error)
	return c, nil
}

// decline states a refusal of commerce in the spec's message vocabulary: a token that is not
// a usable shared payment token is invalid, a bank that wants authentication is an
// intervention this seller cannot run, and every other refusal is a declined payment.
func decline(e *billing.AgentError) *Refusal {
	switch e.Code {
	case "token_invalid":
		return &Refusal{Code: "invalid", Param: "$.payment_data.instrument.credential.token", Resolution: "recoverable", Message: e.Message}
	case "price_changed":
		return &Refusal{Code: "conflict", Resolution: "requires_buyer_review",
			Message: "The price changed since the checkout session was quoted. Update the session to see the new price."}
	case "authentication_required":
		return &Refusal{Code: "intervention_required", Resolution: "requires_buyer_input",
			Message: "The card's bank requires authentication that this seller cannot complete for an agent. The buyer must pay on the seller's site."}
	}
	return &Refusal{Code: "payment_declined", Resolution: "recoverable", Message: e.Message}
}

// refused states a commerce failure as the fault the agent is answered with. The messages
// of commerce are written to be shown to the caller.
func refused(f *billing.PaymentFault) error {
	switch {
	case f.Status == http.StatusUnauthorized:
		return unauthorized(f.Message)
	case f.Status == http.StatusNotFound:
		return NotFound(f.Message)
	case f.Status == http.StatusConflict:
		return Conflict(f.Message)
	case f.Status == http.StatusUnprocessableEntity:
		return Unsupported("", f.Message)
	case f.Status == http.StatusTooManyRequests:
		return fault(http.StatusTooManyRequests, TypeInvalidRequest, "rate_limited", f.Message, "")
	case f.Status >= http.StatusInternalServerError:
		return Unavailable(f.Message)
	}
	return Invalid("", f.Message)
}

// Durable is a Ledger kept in commerce's datastore (models/idempotencykey), so a key is held
// across restarts and across every node. The record outlives Retention. A claim carries a
// token of its own: after a guard that stalled past its TTL is reclaimed, the first request
// can neither release nor settle the guard of the one that reclaimed it.
type Durable struct{}

// Begin implements Ledger.
func (Durable) Begin(ctx context.Context, scope, key, digest string) (Verdict, Stored, Lease, error) {
	db := datastore.New(nscontext.WithNamespace(ctx, "system"))
	rec, replay, err := idempotencykey.Begin(db, scope, key)
	if err != nil {
		return 0, Stored{}, nil, err
	}
	if !replay {
		var b [12]byte
		if _, err := rand.Read(b[:]); err != nil {
			_ = rec.Delete()
			return 0, Stored{}, nil, err
		}
		token := hex.EncodeToString(b[:])
		rec.Holder = token
		switch ok, err := rec.Swap(); {
		case err != nil:
			return 0, Stored{}, nil, err
		case !ok:
			// The row changed under the claim: another request holds it.
			return Busy, Stored{}, nil, nil
		}
		return Fresh, Stored{}, &durable{db: db, scope: scope, key: key, digest: digest, token: token}, nil
	}
	if rec.Status != idempotencykey.StatusCompleted {
		return Busy, Stored{}, nil, nil
	}
	if rec.Facts != digest {
		return Mismatch, Stored{}, nil, nil
	}
	var kept struct {
		Status int             `json:"status"`
		Body   json.RawMessage `json:"body"`
	}
	if err := json.Unmarshal([]byte(rec.Response), &kept); err != nil {
		return 0, Stored{}, nil, fmt.Errorf("acp: the stored response of an idempotency key is unreadable: %w", err)
	}
	return Replay, Stored{Status: kept.Status, Body: kept.Body}, nil, nil
}

type durable struct {
	db            *datastore.Datastore
	scope, key    string
	digest, token string
}

// errTakenOver is a settle by a request whose guard another request has reclaimed.
var errTakenOver = errors.New("acp: the idempotency guard was reclaimed by another request")

// mine reads the guard and reports whether it is still this request's.
func (d *durable) mine() (*idempotencykey.IdempotencyKey, bool) {
	cur := idempotencykey.New(d.db)
	if err := cur.Get(d.db.NewKey(cur.Kind(), idempotencykey.DeterministicID(d.scope, d.key), 0, nil)); err != nil {
		return nil, false
	}
	return cur, cur.Holder == d.token
}

func (d *durable) Settle(_ context.Context, res Stored) error {
	b, err := json.Marshal(struct {
		Status int             `json:"status"`
		Body   json.RawMessage `json:"body"`
	}{res.Status, res.Body})
	if err != nil {
		return err
	}
	cur, ok := d.mine()
	if !ok {
		return errTakenOver
	}
	cur.Facts = d.digest
	return idempotencykey.Complete(cur, string(b))
}

// Release drops the guard, only while it is this request's.
func (d *durable) Release(_ context.Context) error {
	if cur, ok := d.mine(); ok {
		return cur.Delete()
	}
	return nil
}

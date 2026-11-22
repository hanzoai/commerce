// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package ucp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/hanzoai/commerce/api/billing"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/checkoutsession"
	"github.com/hanzoai/commerce/models/idempotencykey"
	"github.com/hanzoai/commerce/models/organization"
	orgpkg "github.com/hanzoai/commerce/pkg/org"
	"github.com/hanzoai/commerce/util/nscontext"
)

// Commerce is a Backend, and an Orders, over commerce's headless purchase surface
// (api/billing/agent.go). A line's item.id is the catalog's, `credit-<cents>` or a
// plan's slug, and its quantity is 1. The instrument of the handler named by
// Handler carries a Stripe shared payment token in credential.token.
type Commerce struct {
	// Handler is the id of the payment handler instance, in the profile's
	// payment_handlers, whose instruments carry shared payment tokens.
	Handler string
	// Permalink returns the absolute https URL of an order on the merchant's
	// site (schemas/shopping/types/order_confirmation.json).
	Permalink func(orderID string) string
	// Continue returns the absolute https URL where a buyer finishes a checkout
	// the bank asked to authenticate. Without it that case is answered as a
	// recoverable payment_failed.
	Continue func(checkoutID string) string

	orgs     func(ctx context.Context, name string) (*organization.Organization, error)
	complete func(ctx context.Context, org *organization.Organization, subject, host, id, key string, pay billing.PaymentIn) billing.CompleteOut
}

// NewCommerce returns c bound to commerce's org resolver and CompleteCheckout.
func NewCommerce(c Commerce) (*Commerce, error) {
	if c.Permalink == nil {
		return nil, errors.New("ucp: Commerce needs Permalink")
	}
	c.orgs, c.complete = orgpkg.Resolve, billing.CompleteCheckout
	return &c, nil
}

// planInfo is what the live catalog says of a plan.
type planInfo struct {
	Name  string
	Price int64
}

// livePlans reads the plans the catalog sells on host.
func livePlans(ctx context.Context, host string) (map[string]planInfo, error) {
	ps, err := billing.ReadPlans(ctx, host, "", nil)
	if err != nil {
		return nil, err
	}
	out := make(map[string]planInfo, len(ps))
	for _, p := range ps {
		out[p.Slug] = planInfo{Name: p.Name, Price: p.Price}
	}
	return out, nil
}

const creditPrefix = "credit-"

// parseItem maps a UCP item id to what the headless surface sells: the credit
// pack `credit-<cents>`, else a plan by slug.
func parseItem(id string) (billing.ItemIn, bool) {
	if rest, ok := strings.CutPrefix(id, creditPrefix); ok {
		if cents, err := strconv.ParseInt(rest, 10, 64); err == nil && cents > 0 && strconv.FormatInt(cents, 10) == rest {
			return billing.ItemIn{Kind: checkoutsession.Credit, UsdCents: cents}, true
		}
	}
	return billing.ItemIn{Kind: checkoutsession.Plan, Plan: id}, id != ""
}

// itemID is the UCP item id of what a checkout line sells.
func itemID(it billing.ItemIn) string {
	if it.Kind == checkoutsession.Plan {
		return it.Plan
	}
	return creditPrefix + strconv.FormatInt(it.UsdCents, 10)
}

// lines maps the line items of a request to the one item a checkout sells. A
// problem comes back as the message that tells the platform what to change.
func lines(ls []LineItem) (billing.ItemIn, *Message) {
	if len(ls) != 1 {
		m := Err("too_many_items", Unrecoverable, "$.line_items", "a checkout sells exactly one item")
		return billing.ItemIn{}, &m
	}
	l := ls[0]
	if l.Quantity != 1 {
		m := Err("quantity_unsupported", Unrecoverable, "$.line_items[0].quantity", "item %s is sold one at a time", l.Item.ID)
		return billing.ItemIn{}, &m
	}
	if qu := l.Item.QuantityUnit; qu != nil {
		scale := 0
		if qu.Scale != nil {
			scale = *qu.Scale
		}
		if qu.Unit != "C62" || scale != 0 {
			m := Err("quantity_unit_mismatch", Unrecoverable, "$.line_items[0].item.quantity_unit",
				"item %s is sold by the each; the quantity is a count of whole items", l.Item.ID)
			return billing.ItemIn{}, &m
		}
	}
	it, ok := parseItem(l.Item.ID)
	if !ok {
		m := Err("item_unavailable", Unrecoverable, "$.line_items[0].item.id", "item %s cannot be purchased", l.Item.ID)
		return billing.ItemIn{}, &m
	}
	return it, nil
}

func (c *Commerce) org(ctx context.Context, who Caller) (*organization.Organization, error) {
	if who.Org == "" || who.Subject == "" {
		return nil, ErrUnauthorized
	}
	o, err := c.orgs(ctx, who.Org)
	if err != nil || o == nil {
		return nil, ErrUnauthorized
	}
	return o, nil
}

// fault maps a billing refusal that is not a business outcome to the error a
// Backend returns.
func fault(f *billing.PaymentFault) error {
	switch f.Status {
	case http.StatusUnauthorized:
		return ErrUnauthorized
	case http.StatusNotFound:
		return ErrNotFound
	case http.StatusServiceUnavailable, http.StatusBadGateway:
		return &Unavailable{}
	}
	return f
}

// pricing says a request's item was refused by the catalog.
func pricing(f *billing.PaymentFault) (*Message, bool) {
	switch f.Status {
	case http.StatusBadRequest, http.StatusUnprocessableEntity, http.StatusNotFound:
		m := Err("item_unavailable", Unrecoverable, "$.line_items[0].item.id", "%s", f.Message)
		return &m, true
	}
	return nil, false
}

// Create implements Backend.
func (c *Commerce) Create(ctx context.Context, who Caller, in *CreateRequest) (*Checkout, error) {
	org, err := c.org(ctx, who)
	if err != nil {
		return nil, err
	}
	it, msg := lines(in.LineItems)
	if msg != nil {
		return nil, &Refusal{Messages: []Message{*msg}}
	}
	v, f := billing.CreateCheckout(ctx, org, who.Subject, who.Host, []billing.ItemIn{it})
	if f != nil {
		if m, ok := pricing(f); ok {
			return nil, &Refusal{Messages: []Message{*m}}
		}
		return nil, fault(f)
	}
	return c.checkout(ctx, who, v)
}

// Read implements Backend.
func (c *Commerce) Read(ctx context.Context, who Caller, id string) (*Checkout, error) {
	org, err := c.org(ctx, who)
	if err != nil {
		return nil, err
	}
	v, f := billing.ReadCheckout(ctx, org, who.Subject, id)
	if f != nil {
		return nil, fault(f)
	}
	return c.checkout(ctx, who, v)
}

// Update implements Backend.
func (c *Commerce) Update(ctx context.Context, who Caller, id string, in *UpdateRequest) (*Checkout, error) {
	org, err := c.org(ctx, who)
	if err != nil {
		return nil, err
	}
	cur, err := c.Read(ctx, who, id)
	if err != nil {
		return nil, err
	}
	it, msg := lines(in.LineItems)
	if msg != nil {
		msg.Severity = Recoverable
		return nil, &Refusal{Current: cur, Messages: []Message{*msg}}
	}
	v, f := billing.UpdateCheckout(ctx, org, who.Subject, who.Host, id, []billing.ItemIn{it})
	switch {
	case f == nil:
		return c.checkout(ctx, who, v)
	case f.Status == http.StatusConflict:
		return nil, &Refusal{Current: cur, Messages: []Message{Err("not_modifiable", Unrecoverable, "", "%s", f.Message)}}
	}
	if m, ok := pricing(f); ok {
		m.Severity = Recoverable
		return nil, &Refusal{Current: cur, Messages: []Message{*m}}
	}
	return nil, fault(f)
}

// Complete implements Backend. The payment is the one instrument of the request:
// a shared payment token, charged through CompleteCheckout, which binds the
// charge to the checkout, the buyer and key.
func (c *Commerce) Complete(ctx context.Context, who Caller, id, key string, in *CompleteRequest) (*Checkout, error) {
	org, err := c.org(ctx, who)
	if err != nil {
		return nil, err
	}
	cur, err := c.Read(ctx, who, id)
	if err != nil {
		return nil, err
	}
	refuse := func(path, format string, args ...any) error {
		cur.Status = Incomplete
		return &Refusal{Current: cur, Messages: []Message{Err("payment_failed", Recoverable, path, format, args...)}}
	}
	if in.Payment == nil || len(in.Payment.Instruments) != 1 {
		return nil, refuse("$.payment.instruments", "A checkout is paid with exactly one payment instrument")
	}
	inst := in.Payment.Instruments[0]
	if c.Handler == "" || inst.HandlerID != c.Handler {
		return nil, refuse("$.payment.instruments[0].handler_id", "Payment handler %q cannot be honoured for this checkout", inst.HandlerID)
	}
	if inst.Credential == nil || inst.Credential.Token == "" {
		return nil, refuse("$.payment.instruments[0].credential", "The instrument carries no shared payment token")
	}
	out := c.complete(ctx, org, who.Subject, who.Host, id, key, billing.PaymentIn{Type: "shared_payment_token", Token: inst.Credential.Token})
	if out.View == nil {
		return nil, &Unavailable{}
	}
	return c.outcome(ctx, who, out)
}

// outcome maps what CompleteCheckout answered. A refusal of the payment leaves
// the checkout usable, which UCP reports as incomplete with a recoverable
// payment_failed; the buyer's bank asking to authenticate is a handoff.
func (c *Commerce) outcome(ctx context.Context, who Caller, out billing.CompleteOut) (*Checkout, error) {
	v := out.View
	if v.Error == nil || v.Status == checkoutsession.Complete {
		return c.checkout(ctx, who, v)
	}
	switch out.Status {
	case http.StatusServiceUnavailable, http.StatusBadGateway:
		return nil, &Unavailable{}
	case http.StatusNotFound:
		return nil, ErrNotFound
	}
	if v.Error.Code == "price_changed" {
		cur, err := c.Read(ctx, who, v.ID)
		if err != nil {
			return nil, err
		}
		cur.Status = Incomplete
		return nil, &Refusal{Current: cur, Messages: []Message{Err("totals_changed", Recoverable, "$.totals", "%s", v.Error.Message)}}
	}
	if v.Error.Code == "already_completed" {
		// Another completion won the checkout while this one was on its way.
		cur, err := c.Read(ctx, who, v.ID)
		if err != nil {
			return nil, err
		}
		return nil, &Refusal{Current: cur, Messages: []Message{Err("not_modifiable", Unrecoverable, "", "%s", v.Error.Message)}}
	}
	return c.checkout(ctx, who, v)
}

// Cancel implements Backend. A checkout that is open, or whose payment was
// refused, becomes canceled; one whose payment is in flight is not canceled.
func (c *Commerce) Cancel(ctx context.Context, who Caller, id string) (*Checkout, error) {
	org, err := c.org(ctx, who)
	if err != nil {
		return nil, err
	}
	cur, err := c.Read(ctx, who, id)
	if err != nil {
		return nil, err
	}
	_, changed, err := checkoutsession.Change(id, func(s *checkoutsession.Session) bool {
		if !strings.EqualFold(s.Subject, who.Subject) || !strings.EqualFold(s.Org, org.Name) {
			return false
		}
		if s.Status != checkoutsession.Open && s.Status != checkoutsession.Failed {
			return false
		}
		s.Status, s.Reason = checkoutsession.Canceled, ""
		return true
	})
	if err != nil {
		return nil, &Unavailable{}
	}
	if !changed {
		return nil, &Refusal{Current: cur, Messages: []Message{Err("not_cancelable", Recoverable, "", "the checkout is %s and cannot be canceled now", cur.Status)}}
	}
	return c.Read(ctx, who, id)
}

// Order implements Orders: the order a completed checkout placed. Credit and
// plan are provisioned when the payment is taken, so the line is fulfilled.
func (c *Commerce) Order(ctx context.Context, who Caller, id string) (*Order, error) {
	org, err := c.org(ctx, who)
	if err != nil {
		return nil, err
	}
	ov, f := billing.ReadOrder(ctx, org, who.Subject, id)
	if f != nil {
		return nil, fault(f)
	}
	return c.orderOf(ctx, who, ov)
}

// orderOf renders an order for the webhook that announces it.
func (c *Commerce) orderOf(ctx context.Context, who Caller, ov *billing.OrderView) (*Order, error) {
	it := billing.ItemIn{Kind: checkoutsession.Credit, UsdCents: ov.CreditedCents}
	if ov.Plan != "" {
		it = billing.ItemIn{Kind: checkoutsession.Plan, Plan: ov.Plan}
	}
	currency := strings.ToUpper(ov.Currency)
	item, totals := c.quote(ctx, who, it, ov.TotalCents, currency)
	one := int64(1)
	return &Order{
		ID: ov.ID, CheckoutID: ov.CheckoutID, PermalinkURL: c.Permalink(ov.ID), Currency: currency,
		LineItems: []OrderLine{{ID: item.ID, Item: item, Quantity: OrderQuantity{Original: &one, Total: 1, Fulfilled: 1}, Totals: totals, Status: LineFulfilled}},
		Totals:    totals,
	}, nil
}

// quote is the item and the totals of a one-line checkout or order of cents. A
// plan is titled by the catalog and credit by its amount.
func (c *Commerce) quote(ctx context.Context, who Caller, it billing.ItemIn, cents int64, currency string) (Item, []Total) {
	title := fmt.Sprintf("%s %d.%02d", currency, it.UsdCents/100, it.UsdCents%100)
	if it.Kind == checkoutsession.Plan {
		title = it.Plan
		if live, err := livePlans(ctx, who.Host); err == nil && live[it.Plan].Name != "" {
			title = live[it.Plan].Name
		}
	}
	return Item{ID: itemID(it), Title: title, Price: &cents}, []Total{{Type: "subtotal", Amount: cents}, {Type: "total", Amount: cents}}
}

// checkout renders a checkout view as a UCP checkout (without the envelope).
func (c *Commerce) checkout(ctx context.Context, who Caller, v *billing.CheckoutView) (*Checkout, error) {
	if len(v.Items) != 1 {
		return nil, fmt.Errorf("checkout %s has %d items", v.ID, len(v.Items))
	}
	currency := strings.ToUpper(v.Currency)
	item, totals := c.quote(ctx, who, v.Items[0], v.TotalCents, currency)
	out := &Checkout{
		ID: v.ID, Currency: currency,
		LineItems: []LineItem{{ID: item.ID, Item: item, Quantity: 1, Totals: totals}},
		Totals:    totals,
	}
	switch v.Status {
	case checkoutsession.Complete:
		out.Status = Completed
		if v.Order != nil {
			out.Order = &OrderRef{ID: v.Order.ID, PermalinkURL: c.Permalink(v.Order.ID)}
		}
		return out, nil
	case checkoutsession.Canceled:
		out.Status = Canceled
		return out, nil
	case checkoutsession.Pending:
		out.Status = CompleteInProgress
	default:
		out.Status = ReadyForComplete
	}
	if v.Error != nil {
		c.refusal(out, v)
	}
	if out.Status != Canceled && out.Status != Completed && out.ContinueURL == "" && c.Continue != nil {
		out.ContinueURL = c.Continue(v.ID)
	}
	return out, nil
}

// refusal reports why the last completion was refused. Every payment refusal is
// payment_failed, the standard error platforms handle with payment-method UX;
// the bank's request to authenticate is a handoff to the buyer when there is
// somewhere to hand off to.
func (c *Commerce) refusal(out *Checkout, v *billing.CheckoutView) {
	e := v.Error
	if e.Code == "in_progress" {
		// A completion is under way: the checkout is being completed, nothing failed.
		return
	}
	if e.Code == "authentication_required" && c.Continue != nil {
		out.Status = RequiresEscalation
		out.ContinueURL = c.Continue(v.ID)
		out.Messages = append(out.Messages, Err("requires_3ds", RequiresBuyerInput, "", "%s", e.Message))
		return
	}
	out.Status = Incomplete
	out.Messages = append(out.Messages, Err("payment_failed", Recoverable, "$.payment.instruments[0]", "%s (%s)", e.Message, e.Code))
}

// Idempotency is a Store over commerce's idempotency guard
// (models/idempotencykey) with records kept for Keep: an older key is a new key,
// and Sweep deletes the ones nothing asks about.
//
// The guards live in the system namespace beside other packages' — billing's
// payment-webhook and intent guards, the ACP, AP2 and MPP ledgers — and a webhook
// guard deleted is a processor's retry processed twice. So every UCP record is
// kept under the one scope idempotencyScope, with the request's own scope folded
// into its key (guard), and Sweep asks for that scope by equality: the same answer
// on every engine and collation, where a prefix range over the request scopes
// would sort by the database's collation.
type Idempotency struct {
	clock func() time.Time
}

func (s Idempotency) db(ctx context.Context) *datastore.Datastore {
	return datastore.New(nscontext.WithNamespace(ctx, "system"))
}

func (s Idempotency) now() time.Time {
	if s.clock != nil {
		return s.clock()
	}
	return time.Now()
}

// idempotencyScope is the scope every UCP record is kept under, and the only one
// Sweep reads.
const idempotencyScope = "ucp"

// guard is the key a UCP record is kept under: the request's scope and its
// Idempotency-Key, so a key is still unique per scope.
func guard(scope, key string) string { return scope + "\x00" + key }

// Begin implements Store.
func (s Idempotency) Begin(ctx context.Context, scope, key, digest string) (*Seen, error) {
	db := s.db(ctx)
	for range 2 {
		rec, replay, err := idempotencykey.Begin(db, idempotencyScope, guard(scope, key))
		if err != nil {
			return nil, err
		}
		if !replay {
			rec.Facts = digest
			return nil, rec.Put()
		}
		if !rec.CreatedAt.IsZero() && s.now().Sub(rec.CreatedAt) >= Keep {
			if err := rec.Delete(); err != nil {
				return nil, err
			}
			continue
		}
		seen := &Seen{Digest: rec.Facts, Done: rec.Status == idempotencykey.StatusCompleted}
		if seen.Digest == "" {
			// Reserved by a request that has not yet recorded its digest: in flight.
			seen.Digest = digest
		}
		if seen.Done {
			if err := json.Unmarshal([]byte(rec.Response), &seen.Response); err != nil {
				return nil, err
			}
		}
		return seen, nil
	}
	return nil, errors.New("ucp: an idempotency record kept expiring under the request")
}

// Sweep deletes up to limit UCP records older than Keep, oldest first, and returns
// how many it deleted. It reads UCP's records alone: another package's guard in
// the namespace is never deleted, and never takes a place under limit.
func (s Idempotency) Sweep(ctx context.Context, limit int) (int, error) {
	db := s.db(ctx)
	var recs []*idempotencykey.IdempotencyKey
	keys, err := idempotencykey.Query(db).Filter("Scope=", idempotencyScope).Order("CreatedAt").Limit(limit).GetAll(&recs)
	if err != nil {
		return 0, err
	}
	n := 0
	for i, rec := range recs {
		if rec.Scope != idempotencyScope {
			// The filter already said so; a row of another scope is never ours to delete.
			continue
		}
		if rec.CreatedAt.IsZero() || s.now().Sub(rec.CreatedAt) < Keep {
			break
		}
		if err := db.Delete(keys[i]); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

// Finish implements Store.
func (s Idempotency) Finish(ctx context.Context, scope, key, digest string, r Stored) error {
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	rec := idempotencykey.New(s.db(ctx))
	rec.Scope, rec.IdemKey, rec.Facts = idempotencyScope, guard(scope, key), digest
	return idempotencykey.Complete(rec, string(b))
}

// Release implements Store.
func (s Idempotency) Release(ctx context.Context, scope, key string) error {
	db := s.db(ctx)
	rec := idempotencykey.New(db)
	err := rec.Get(db.NewKey(rec.Kind(), idempotencykey.DeterministicID(idempotencyScope, guard(scope, key)), 0, nil))
	switch {
	case errors.Is(err, datastore.ErrNoSuchEntity):
		return nil
	case err != nil:
		return err
	}
	return rec.Delete()
}

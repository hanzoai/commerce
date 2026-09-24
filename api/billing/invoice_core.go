// Copyright 2014-present Hanzo AI Inc. Licensed under MIT OR Apache-2.0.

package billing

// The INVOICE LIFECYCLE, as cores that take values instead of requests.
//
// Same split, same reason as payment_core.go: create, issue, collect and void
// were reachable only through a *zip.Ctx, so the whole outbound-billing lifecycle
// was invisible to the registry that yields the OpenAPI operation, the MCP tool,
// the SDK method and the CLI command. An agent could read invoices and download
// their PDFs and could not raise one.
//
// The lifecycle is a STATE MACHINE and the model owns it (billinginvoice's
// Finalize / MarkPaid / MarkVoid each refuse from the wrong state), so these
// functions deliberately do not re-check what the model already refuses. They
// resolve the row, ask the model to move, persist, and emit — and where the model
// says no, that no is the caller's 400.
//
// COLLECTION IS NOT A SEPARATE PAYMENT PATH. A subscription's invoice is paid
// the way the billing cycle pays it — from the source the subscription was
// bought from, on the attempt recorded on the invoice — and any other invoice by
// the engine's prepaid waterfall. What lives here is the lock and the emit around
// it; nothing about how money moves is restated.

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/hanzoai/commerce/billing/engine"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/events"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/thirdparty/kms"
	"github.com/hanzoai/commerce/types"
	"github.com/zap-proto/zip"
)

// InvoiceLine is one charge on an invoice.
type InvoiceLine struct {
	// Description is the human-readable line, e.g. "Advisory retainer — August".
	Description string `json:"description"`
	// Amount is the line total in whole cents.
	Amount int64 `json:"amount"`
	// Quantity is the number of units, when the line is metered. Optional.
	Quantity int64 `json:"quantity,omitempty"`
	// UnitPrice is the per-unit price in cents, when the line is metered. Optional.
	UnitPrice int64 `json:"unitPrice,omitempty"`
}

// InvoiceIn is a draft invoice to raise.
type InvoiceIn struct {
	// UserID is the customer this invoice bills. Required.
	UserID string
	// CustomerEmail is where the invoice is sent. Optional.
	CustomerEmail string
	// Currency is the ISO code. Empty means the model default (usd).
	Currency string
	// Lines are the charges. The subtotal and amount due are computed from them.
	Lines []InvoiceLine
}

// InvoiceView is an invoice as a TYPED value. The HTTP handlers answer with a
// map built by invoiceResponse; this is the same facts with a schema, which is
// what lets the lifecycle be published as tools an agent can actually read.
type InvoiceView struct {
	// ID is the invoice id.
	ID string `json:"id"`
	// Number is the human-facing invoice number, e.g. "INV-0042". Empty until issued.
	Number string `json:"number,omitempty"`
	// UserID is the customer billed.
	UserID string `json:"userId"`
	// CustomerEmail is where it is sent.
	CustomerEmail string `json:"customerEmail,omitempty"`
	// Status is draft, open, paid, void or uncollectible.
	Status string `json:"status"`
	// Currency is the ISO code.
	Currency string `json:"currency"`
	// SubtotalCents is the sum of the lines, before tax, discount and credit.
	SubtotalCents int64 `json:"subtotalCents"`
	// AmountDueCents is what remains collectible.
	AmountDueCents int64 `json:"amountDueCents"`
	// AmountPaidCents is what has been collected so far.
	AmountPaidCents int64 `json:"amountPaidCents"`
	// Lines are the charges on the invoice.
	Lines []InvoiceLine `json:"lines,omitempty"`
	// PaymentRef is the processor reference for the collection, once paid.
	PaymentRef string `json:"paymentRef,omitempty"`
	// CreatedAt is when the draft was raised, RFC3339.
	CreatedAt string `json:"createdAt,omitempty"`
}

// InvoiceCollection is the outcome of attempting to collect an invoice.
type InvoiceCollection struct {
	// Invoice is the invoice after the attempt — the authoritative status.
	Invoice *InvoiceView `json:"invoice"`
	// Paid reports whether the invoice is now settled in full.
	Paid bool `json:"paid"`
	// CreditUsedCents is how much was covered by credit grants.
	CreditUsedCents int64 `json:"creditUsedCents"`
	// BalanceUsedCents is how much was covered by prepaid balance.
	BalanceUsedCents int64 `json:"balanceUsedCents"`
	// CardChargedCents is how much was charged to the card on file.
	CardChargedCents int64 `json:"cardChargedCents"`
	// ProcessorRef is the processor's reference for any card charge.
	ProcessorRef string `json:"processorRef,omitempty"`
	// Reason explains a decline or partial collection. Empty on success.
	Reason string `json:"reason,omitempty"`
}

// viewInvoice projects the model onto the typed view. ONE projection, so the
// lifecycle ops cannot describe the same invoice two ways.
func viewInvoice(inv *billinginvoice.BillingInvoice) *InvoiceView {
	v := &InvoiceView{
		ID:              inv.Id(),
		Number:          inv.NumberStr,
		UserID:          inv.UserId,
		CustomerEmail:   inv.CustomerEmail,
		Status:          string(inv.Status),
		Currency:        string(inv.Currency),
		SubtotalCents:   inv.Subtotal,
		AmountDueCents:  inv.AmountDue,
		AmountPaidCents: inv.AmountPaid,
		PaymentRef:      inv.PaymentRef,
	}
	for _, li := range inv.LineItems {
		v.Lines = append(v.Lines, InvoiceLine{
			Description: li.Description,
			Amount:      li.Amount,
			Quantity:    li.Quantity,
			UnitPrice:   li.UnitPrice,
		})
	}
	if !inv.CreatedAt.IsZero() {
		v.CreatedAt = inv.CreatedAt.UTC().Format(time.RFC3339)
	}
	return v
}

// loadInvoice resolves one invoice inside the org's own namespace. The namespace
// IS the tenant boundary: an id from another org is not found rather than found
// and then filtered, so there is no window in which the wrong row is in hand.
func loadInvoice(ctx context.Context, org *organization.Organization, id string) (*billinginvoice.BillingInvoice, *datastore.Datastore, *PaymentFault) {
	if org == nil {
		return nil, nil, fault(401, "missing identity headers", nil)
	}
	if strings.TrimSpace(id) == "" {
		return nil, nil, fault(400, "invoice id is required", nil)
	}
	db := datastore.New(org.Namespaced(ctx))
	inv := billinginvoice.New(db)
	if err := inv.GetById(strings.TrimSpace(id)); err != nil {
		return nil, nil, fault(404, "invoice not found", err)
	}
	return inv, db, nil
}

// RaiseInvoice creates a DRAFT invoice against the org's customer.
//
// A draft is deliberately not collectible: it exists to be edited and then
// issued, and IssueInvoice is what makes it a demand for payment. Raising and
// issuing are separate acts because an invoice that became collectible the moment
// it was created could be collected before anyone had read it.
func RaiseInvoice(ctx context.Context, org *organization.Organization, in InvoiceIn) (*InvoiceView, *PaymentFault) {
	inv, f := raiseInvoice(ctx, org, in)
	if f != nil {
		return nil, f
	}
	return viewInvoice(inv), nil
}

// raiseInvoice is the worker: it returns the MODEL, so the HTTP handler can render
// its long-standing invoiceResponse body while the typed op renders InvoiceView.
// Two projections of one result, never two implementations.
func raiseInvoice(ctx context.Context, org *organization.Organization, in InvoiceIn) (*billinginvoice.BillingInvoice, *PaymentFault) {
	if org == nil {
		return nil, fault(401, "missing identity headers", nil)
	}
	if strings.TrimSpace(in.UserID) == "" {
		return nil, fault(400, "userId is required", nil)
	}
	db := datastore.New(org.Namespaced(ctx))
	inv := billinginvoice.New(db)
	inv.UserId = strings.TrimSpace(in.UserID)
	inv.CustomerEmail = strings.TrimSpace(in.CustomerEmail)
	if c := normalizeCurrency(in.Currency); c != "" {
		inv.Currency = c
	}
	for _, l := range in.Lines {
		inv.LineItems = append(inv.LineItems, billinginvoice.LineItem{
			Description: l.Description,
			Amount:      l.Amount,
			Quantity:    l.Quantity,
			UnitPrice:   l.UnitPrice,
		})
	}
	// The subtotal is COMPUTED from the lines, never taken from the caller: a
	// caller-supplied total that disagreed with its own lines is an invoice that
	// bills a number nobody can derive.
	inv.RecalculateSubtotal()
	if err := inv.Create(); err != nil {
		log.Error("Failed to create invoice: %v", err)
		return nil, fault(500, "failed to create invoice", err)
	}
	return inv, nil
}

// ReadInvoice returns one invoice from the org's own namespace.
func ReadInvoice(ctx context.Context, org *organization.Organization, id string) (*InvoiceView, *PaymentFault) {
	inv, _, f := loadInvoice(ctx, org, id)
	if f != nil {
		return nil, f
	}
	return viewInvoice(inv), nil
}

// IssueInvoice moves a draft to OPEN — the act that makes it collectible and
// gives it its number.
//
// The model refuses from any state but draft, and that refusal is the caller's
// 400 rather than something re-checked here: one state machine, in one place.
func IssueInvoice(ctx context.Context, org *organization.Organization, id string, ev *events.Client) (*InvoiceView, *PaymentFault) {
	inv, f := issueInvoice(ctx, org, id, ev)
	if f != nil {
		return nil, f
	}
	return viewInvoice(inv), nil
}

// issueInvoice is the worker. See raiseInvoice.
func issueInvoice(ctx context.Context, org *organization.Organization, id string, ev *events.Client) (*billinginvoice.BillingInvoice, *PaymentFault) {
	inv, _, f := loadInvoice(ctx, org, id)
	if f != nil {
		return nil, f
	}
	if err := inv.Finalize(); err != nil {
		return nil, fault(400, err.Error(), nil)
	}
	if err := inv.Update(); err != nil {
		log.Error("Failed to finalize invoice: %v", err)
		return nil, fault(500, "failed to finalize invoice", err)
	}
	emitInvoiceCtx(ctx, ev, org.Name, inv, evInvoiceFinalized)
	return inv, nil
}

// VoidInvoiceIn cancels a draft or open invoice.
func VoidInvoiceIn(ctx context.Context, org *organization.Organization, id string, ev *events.Client) (*InvoiceView, *PaymentFault) {
	inv, f := voidInvoice(ctx, org, id, ev, nil)
	if f != nil {
		return nil, f
	}
	return viewInvoice(inv), nil
}

// operatorVoid is a platform operator voiding an invoice whose payment attempt
// has no known outcome, having settled that attempt with the processor (found it
// never landed, or refunded it there): who, and why.
type operatorVoid struct {
	Actor  string
	Reason string
}

// voidInvoice is the worker. See raiseInvoice. It holds the invoice's lock, so
// it cannot void an invoice a payment is at the processor for, and it gives
// back what the invoice collected (engine.ReturnPaid). It refuses one whose last
// payment attempt has no known outcome, since that money may have moved, unless
// op is an operator taking the attempt over (unresolvedAttempt).
//
// An operator's void records who, why and the abandoned attempt on the invoice
// itself, in the same write that voids it, and then in the billing event ledger
// under a key derived from the invoice (invoice.operator_void). An operator
// retrying a void whose event was not written writes it then, once.
func voidInvoice(ctx context.Context, org *organization.Organization, id string, ev *events.Client, op *operatorVoid) (*billinginvoice.BillingInvoice, *PaymentFault) {
	inv, db, f := loadInvoice(ctx, org, id)
	if f != nil {
		return nil, f
	}
	release, err := engine.LockInvoice(db, inv.Id())
	if errors.Is(err, engine.ErrInvoiceBusy) {
		return nil, fault(409, "a payment on this invoice is in progress", nil)
	}
	if err != nil {
		return nil, fault(500, "failed to lock the invoice", err)
	}
	defer release()
	if err := inv.GetById(inv.Id()); err != nil {
		return nil, fault(404, "invoice not found", err)
	}
	if recorded := operatorVoidOf(inv); op != nil && inv.Status == billinginvoice.Void && recorded != nil {
		if f := recordOperatorVoid(db, inv, recorded); f != nil {
			return nil, f
		}
		return inv, nil
	}
	if inv.PendingKey != "" {
		if op == nil {
			return nil, fault(409, "a payment attempt on this invoice has no known outcome yet; it cannot be voided until it resolves", nil)
		}
		if f := unresolvedAttempt(db, inv); f != nil {
			return nil, f
		}
	}
	attempt := types.Map{"key": inv.PendingKey, "method": inv.PendingMethod, "amount": inv.PendingAmount, "ref": inv.PendingRef}
	if err := inv.MarkVoid(); err != nil {
		return nil, fault(400, err.Error(), nil)
	}
	var recorded types.Map
	if op != nil {
		inv.PendingMethod, inv.PendingAmount, inv.PendingKey, inv.PendingRef = "", 0, "", ""
		recorded = types.Map{"actor": op.Actor, "reason": op.Reason, "subscriptionId": inv.SubscriptionId,
			"amountDue": inv.AmountDue, "amountPaid": inv.AmountPaid, "attempt": attempt}
		if inv.Metadata == nil {
			inv.Metadata = types.Map{}
		}
		inv.Metadata["operatorVoid"] = recorded
	}
	if err := engine.ReturnPaid(ctx, db, inv, prepaidFor(ctx, org), inv.VoidedAt); err != nil {
		log.Error("Failed to return what invoice %s collected: %v", inv.Id(), err)
		return nil, fault(500, "failed to return what the invoice collected", err)
	}
	if err := inv.Update(); err != nil {
		log.Error("Failed to void invoice: %v", err)
		return nil, fault(500, "failed to void invoice", err)
	}
	if op != nil {
		log.Info("billing: operator %s voided invoice %s (subscription %s), leaving payment attempt %v with the processor: %s",
			op.Actor, inv.Id(), inv.SubscriptionId, attempt, op.Reason)
		if f := recordOperatorVoid(db, inv, recorded); f != nil {
			return nil, f
		}
	}
	emitInvoiceCtx(ctx, ev, org.Name, inv, evInvoiceVoid)
	return inv, nil
}

// operatorVoidOf is the operator's void recorded on inv, or nil.
func operatorVoidOf(inv *billinginvoice.BillingInvoice) types.Map {
	switch v := inv.Metadata["operatorVoid"].(type) {
	case types.Map:
		return v
	case map[string]interface{}:
		return types.Map(v)
	}
	return nil
}

// recordOperatorVoid writes the operator's void of inv to the billing event
// ledger, once per invoice.
func recordOperatorVoid(db *datastore.Datastore, inv *billinginvoice.BillingInvoice, recorded types.Map) *PaymentFault {
	if _, err := engine.EmitBillingEventOnce(db, "invoice.operator_void:"+inv.Id(), "invoice.operator_void", "invoice", inv.Id(), inv.UserId, recorded); err != nil {
		log.Error("billing: ALERT invoice %s was voided by an operator and the billing event was not written; retry the void to write it: %v", inv.Id(), err)
		return fault(500, "the invoice is void and its billing event was not written; retry to record it", err)
	}
	return nil
}

// unresolvedAttempt allows an operator to take over a payment attempt with no
// known outcome once nothing will settle it: the retry schedule gave up on it
// (its subscription escalated to unpaid), its subscription ended, or it has none.
// While the billing cycle is still repeating it under its key, it is refused.
// What the attempt may have taken at the processor is the operator's to refund
// there; the void returns only what the invoice recorded as collected.
func unresolvedAttempt(db *datastore.Datastore, inv *billinginvoice.BillingInvoice) *PaymentFault {
	if inv.SubscriptionId == "" {
		return nil
	}
	sub := subscription.New(db)
	if err := sub.GetById(inv.SubscriptionId); err != nil {
		if errors.Is(err, datastore.ErrNoSuchEntity) {
			return nil
		}
		return fault(500, "failed to read the invoice's subscription", err)
	}
	if sub.Status == subscription.Unpaid || sub.Status == subscription.Canceled {
		return nil
	}
	return fault(409, "the billing cycle is still repeating this invoice's payment attempt; it can be voided once the attempt is escalated", nil)
}

// CollectInvoice is the customer paying an OPEN invoice themselves: a
// subscription's invoice the way the subscription was bought, any other from
// prepaid money (see collectInvoice). Two payments on one invoice never run at
// once: the second is a 409. An invoice already paid answers as paid.
func CollectInvoice(ctx context.Context, org *organization.Organization, id string, kmsClient *kms.CachedClient, ev *events.Client) (*InvoiceCollection, *PaymentFault) {
	oc, f := collectInvoice(ctx, org, id, kmsClient, ev)
	if f != nil {
		return nil, f
	}
	if oc.Replayed != nil {
		// Already paid: nothing was charged, and this is its receipt.
		return oc.Replayed, nil
	}
	return &InvoiceCollection{
		Invoice:          viewInvoice(oc.Inv),
		Paid:             oc.Result.Success,
		CreditUsedCents:  oc.Result.CreditUsed,
		BalanceUsedCents: oc.Result.BalanceUsed,
		CardChargedCents: oc.Result.ProviderUsed,
		ProcessorRef:     oc.Result.ProviderRef,
		Reason:           oc.Result.Error,
	}, nil
}

// collectOutcome is what the collection worker learned, carried by VALUE.
//
// It exists because a collection has two shapes of answer — a fresh attempt and a
// replay of a sealed one — and both endpoints need to tell them apart. Returning it
// is what keeps that state on the stack: a package-level var would be shared by
// every concurrent collection in the process, which on this path means one
// tenant's receipt answering another tenant's request.
type collectOutcome struct {
	// Inv is the invoice after the attempt.
	Inv *billinginvoice.BillingInvoice
	// Result is the fresh collection outcome. Nil when Replayed is set.
	Result *engine.CollectionResult
	// Replayed is the sealed body of an earlier identical collection. Nil on a
	// fresh attempt.
	Replayed *InvoiceCollection
}

// collectInvoice is the worker. See raiseInvoice.
//
// It holds the invoice's lock (engine.LockInvoice) — the one every path that
// moves money on an invoice holds, the billing cycle included — so a customer's
// payment and a renewal retry can never both be at the processor for one
// invoice; a held lock is a 409. Under it the invoice is read again. A
// subscription's invoice is paid from the source the subscription was bought
// from (renewalOf), on the attempt recorded on the invoice (engine.PayInvoice);
// any other invoice from prepaid money, credits then the balance. A paid invoice
// answers as paid, and nothing is charged.
func collectInvoice(ctx context.Context, org *organization.Organization, id string, kmsClient *kms.CachedClient, ev *events.Client) (*collectOutcome, *PaymentFault) {
	ctx = context.WithoutCancel(ctx)
	inv, db, f := loadInvoice(ctx, org, id)
	if f != nil {
		return nil, f
	}
	release, err := engine.LockInvoice(db, inv.Id())
	if errors.Is(err, engine.ErrInvoiceBusy) {
		return nil, fault(409, "invoice payment already in progress", nil)
	}
	if err != nil {
		return nil, fault(500, "failed to lock the invoice", err)
	}
	defer release()
	if err := inv.GetById(inv.Id()); err != nil {
		return nil, fault(404, "invoice not found", err)
	}
	if inv.Status == billinginvoice.Paid {
		return &collectOutcome{Inv: inv, Replayed: &InvoiceCollection{
			Invoice: viewInvoice(inv), Paid: true, ProcessorRef: inv.PaymentRef,
		}}, nil
	}

	pre := prepaidFor(ctx, org)
	var result *engine.CollectionResult
	if inv.SubscriptionId != "" {
		sub := subscription.New(db)
		if err := sub.GetById(inv.SubscriptionId); err != nil {
			return nil, fault(404, "the invoice's subscription was not found", err)
		}
		c, _, err := renewalOf(org, db, sub, cardFor(kmsClient, org), pre)
		if err != nil {
			return nil, fault(500, "failed to read how the subscription is paid", err)
		}
		if c.Comped || c.Pay == nil {
			return nil, fault(422, "nothing on file pays this subscription's invoice", nil)
		}
		if result, err = engine.PayInvoice(ctx, db, inv, c, time.Now()); err != nil {
			log.Error("Failed to collect invoice %s: %v", inv.Id(), err)
			return nil, fault(500, "failed to collect invoice payment", err)
		}
	} else {
		if result, err = engine.CollectInvoice(ctx, db, inv, pre, nil); err != nil {
			log.Error("Failed to collect invoice: %v", err)
			return nil, fault(500, "failed to collect invoice payment", err)
		}
		if err := inv.Update(); err != nil {
			log.Error("Failed to update invoice after payment: %v", err)
			return nil, fault(500, "failed to update invoice", err)
		}
	}
	if result.Success {
		emitInvoiceCtx(ctx, ev, org.Name, inv, evInvoicePaid)
	}
	return &collectOutcome{Inv: inv, Result: result}, nil
}

// The three invoice lifecycle events, named so emitInvoiceCtx can dispatch on a
// value rather than the caller reaching for a client method directly.
type invoiceEventKind int

const (
	evInvoiceFinalized invoiceEventKind = iota
	evInvoicePaid
	evInvoiceVoid
)

// emitInvoiceCtx is fireEvent for a caller that has a context instead of a
// request. Best-effort and never on the money path's critical line: a missing
// collector is a no-op, exactly as it is for the HTTP handlers.
func emitInvoiceCtx(ctx context.Context, ev *events.Client, orgName string, inv *billinginvoice.BillingInvoice, kind invoiceEventKind) {
	if ev == nil {
		return
	}
	// WithoutCancel (not Background): survive client disconnect but keep trace values.
	detached := context.WithoutCancel(ctx)
	payload := invoiceEvent(orgName, inv)
	go func() {
		switch kind {
		case evInvoiceFinalized:
			ev.EmitInvoiceFinalized(detached, payload)
		case evInvoicePaid:
			ev.EmitInvoicePaid(detached, payload)
		case evInvoiceVoid:
			ev.EmitInvoiceVoid(detached, payload)
		}
	}()
}

// currencyOrEmpty keeps RaiseInvoice from stamping a currency the model would
// otherwise default itself.
var _ = currency.USD

// eventsOf and kmsOf lift the two request-scoped side channels a core takes as
// values off a *zip.Ctx. They exist so the HTTP endpoint — and only the HTTP endpoint —
// knows that these live in locals; the cores take them as parameters and are
// therefore callable from a typed op, a cron, or a test with no request at all.
func eventsOf(c *zip.Ctx) *events.Client {
	if v := c.Locals("events"); v != nil {
		if ev, ok := v.(*events.Client); ok {
			return ev
		}
	}
	return nil
}

func kmsOf(c *zip.Ctx) *kms.CachedClient {
	if v := c.Locals("kms"); v != nil {
		if k, ok := v.(*kms.CachedClient); ok {
			return k
		}
	}
	return nil
}

package billing

// An organization orders a plan by invoice and pays it by wire. The order raises
// an OPEN invoice and activates nothing; the invoice is shared by a link
// (models/invoicelink) that shows it and renders its PDF; staff mark it paid
// against the bank's receipt, which opens the plan and credits its starting
// balance through the payment's record (paymentorg) exactly as a card sale's
// payment does; an invoice unpaid past its due date and grace is voided. A plan
// is sold by invoice when its row states limits.invoiceDays.

import (
	"context"
	"crypto/sha256"
	"encoding/base32"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/hanzoai/commerce/api/promo"
	"github.com/hanzoai/commerce/billing/engine"
	"github.com/hanzoai/commerce/checkout"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/idempotencykey"
	"github.com/hanzoai/commerce/models/invoicelink"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/paymentorg"
	storemodel "github.com/hanzoai/commerce/models/store"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
	types "github.com/hanzoai/commerce/types"
)

// InvoiceNotice is what a buyer who orders by invoice is told wherever the order
// is shown: nothing starts before the money is seen.
const InvoiceNotice = "Your plan starts when we confirm your payment."

// OrderIn is an organization's order of a plan by invoice: the order a card
// sale takes (SubscribeIn), less the card and the coupon. Subject is the
// organization itself, bounded by the caller; Email is the address the invoice is
// sent to. Host is where the buyer ordered, which names the brand whose bank
// receives the wire. Net says the organization is approved to start before it
// pays: the plan opens with the order, and the invoice stays open.
type OrderIn struct {
	Subject        string
	Email          string
	PlanID         string
	Quantity       int
	Lines          []LineIn
	Level          int
	Interval       types.Interval
	BalanceCents   int64
	StoreID        string
	IdempotencyKey string
	Host           string
	Net            bool
	Promo          *promo.Promo
}

// OrderLine is one line of an invoice as its buyer reads it.
type OrderLine struct {
	Description string `json:"description"`
	Quantity    int64  `json:"quantity,omitempty"`
	UnitCents   int64  `json:"unitCents,omitempty"`
	AmountCents int64  `json:"amountCents"`
}

// InvoiceOrder is an order by invoice as its buyer reads it: the invoice, what
// it is for, when it is due and when it is voided if unpaid, how to pay it while
// it is open — the bank with the reference (Wire) and the same one field to a line
// (HowToPay), both read from one live read of the bank — and the link it is shown
// by (only on the answer to the order). Active and SubscriptionID say the plan is
// open: once paid, or at once under net terms (Net). Replaced is the paid plan
// the org held that this one ended when it opened; Payment the wire that paid it
// (WireKey). Cloud's client.InvoiceOrder is this shape, field for field, less
// what cloud keeps to itself (Email, Brand, Link, Net).
type InvoiceOrder struct {
	InvoiceID      string            `json:"invoiceId"`
	Number         string            `json:"number"`
	Reference      string            `json:"reference"`
	Status         string            `json:"status"`
	PlanID         string            `json:"planId"`
	Category       string            `json:"category,omitempty"`
	Seats          int64             `json:"seats"`
	AmountCents    int64             `json:"amountCents"`
	BalanceCents   int64             `json:"balanceCents,omitempty"`
	Currency       string            `json:"currency"`
	IssuedAt       time.Time         `json:"issuedAt"`
	DueAt          time.Time         `json:"dueAt"`
	VoidAt         time.Time         `json:"voidAt"`
	PaidAt         *time.Time        `json:"paidAt,omitempty"`
	VoidedAt       *time.Time        `json:"voidedAt,omitempty"`
	Lines          []OrderLine       `json:"lines"`
	Wire           *WireInstructions `json:"wire,omitempty"`
	HowToPay       []string          `json:"howToPay,omitempty"`
	Link           string            `json:"link,omitempty"`
	Active         bool              `json:"active"`
	SubscriptionID string            `json:"subscriptionId,omitempty"`
	Replaced       string            `json:"replaced,omitempty"`
	Payment        string            `json:"payment,omitempty"`
	Net            bool              `json:"net,omitempty"`
	Email          string            `json:"email,omitempty"`
	Brand          string            `json:"brand"`
	Notice         string            `json:"notice"`
	Unpaid         string            `json:"unpaid"`
}

// order is what an invoice raised for an order keeps of it (Metadata "order"), so
// the invoice alone is enough to show, pay, void or refund it.
type order struct {
	Plan      string         `json:"plan"`
	Level     int            `json:"level,omitempty"`
	Interval  types.Interval `json:"interval"`
	Quantity  int            `json:"quantity"`
	Lines     []LineIn       `json:"lines,omitempty"`
	Seats     int64          `json:"seats"`
	Category  string         `json:"category,omitempty"`
	Balance   int64          `json:"balance,omitempty"`
	Reference string         `json:"reference"`
	Host      string         `json:"host"`
	Promo     int            `json:"promo,omitempty"`
	PromoName string         `json:"promoName,omitempty"`
	Net       bool           `json:"net,omitempty"`
	VoidAt    time.Time      `json:"voidAt"`
	Reason    string         `json:"reason,omitempty"`
	Received  int64          `json:"received,omitempty"`
	// Replaced is the paid plan the org held that this order's plan ended when it
	// opened, as a card sale ends the one it replaces.
	Replaced string `json:"replaced,omitempty"`
	// Reopened is when a wire the bank received before the invoice was voided
	// re-opened it to be paid, and Voided when that void was; Reason keeps why.
	Reopened *time.Time `json:"reopened,omitempty"`
	Voided   *time.Time `json:"voided,omitempty"`
}

// orderOf is the order an invoice was raised for, or false for an invoice that
// was not raised for one.
func orderOf(inv *billinginvoice.BillingInvoice) (*order, bool) {
	raw, ok := inv.Metadata["order"]
	if !ok || inv.PaymentMethod != string(processor.Wire) {
		return nil, false
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return nil, false
	}
	o := new(order)
	if json.Unmarshal(b, o) != nil || o.Plan == "" {
		return nil, false
	}
	return o, true
}

// keep writes o back onto inv's metadata.
func (o *order) keep(inv *billinginvoice.BillingInvoice) {
	if inv.Metadata == nil {
		inv.Metadata = types.Map{}
	}
	inv.Metadata["order"] = o
}

// invoiceScope and invoiceLease name an order's retry guard and the lease every
// change of an order's invoice — paid, voided, expired — is made under, one at a
// time, so a payment and a void that cross are applied one after the other.
const (
	invoiceScope = "billing-invoice:"
	invoiceLease = "billing-invoice-lease:"
)

// expiry is why the sweep voids an invoice unpaid past its due date and grace.
const expiry = "unpaid past due"

// receivedSkew is how far past now a bank's received date may read: its clock
// and ours may disagree by up to a day, never more.
const receivedSkew = 24 * time.Hour

// OrderInvoice raises the invoice for in's order and activates nothing: an OPEN
// invoice of the seats at the row's price and the starting balance, due the row's
// invoiceDays from now, with the link it is shown by and how to pay it. Under net
// terms the plan opens now. It answers the order, and whether it answers an
// earlier identical request (a retry under the same key): the invoice that order
// raised, as it stands now, with a new link and the bank read again, since the
// guard keeps neither a working link nor a bank detail.
//
// It refuses what a card sale refuses (price), and a plan whose row states no
// invoiceDays; a brand with no bank to receive the wire is unavailable. One order
// of a subject is in flight at a time (holdSale), an org that already pays for the
// plan is told so, and an order for a plan with an invoice still open names that
// invoice rather than raise a second. An org that let an order under net terms
// expire unpaid (defaulted) is not started before it pays again: its next order
// opens nothing until it is paid, whatever its terms.
func OrderInvoice(ctx context.Context, org *organization.Organization, in OrderIn) (*InvoiceOrder, bool, error) {
	if org == nil {
		return nil, false, errors.New("invoice: no organization")
	}
	if in.Subject == "" {
		return nil, false, errors.New("invoice: no subject")
	}
	db := datastore.New(org.Namespaced(ctx))
	planID := strings.TrimSpace(in.PlanID)
	if planID == "" {
		return nil, false, saleRefusal{saleRefused, "planId is required"}
	}
	if in.StoreID != "" {
		if err := storemodel.New(db).GetById(in.StoreID); err != nil {
			return nil, false, saleRefusal{saleMissing, "store not found"}
		}
	}
	sale := SubscribeIn{
		Subject: in.Subject, PlanID: planID, Quantity: in.Quantity, Lines: in.Lines, Level: in.Level,
		Interval: in.Interval, BalanceCents: in.BalanceCents, StoreID: in.StoreID, Promo: in.Promo,
	}
	q, err := price(db, org, sale, planID, "")
	if err != nil {
		return nil, false, err
	}
	l := q.plan.Limits
	if l == nil || l.InvoiceDays == nil || *l.InvoiceDays <= 0 {
		return nil, false, coded(saleRefused, "not_sold_by_invoice", fmt.Sprintf("plan %q is not sold by invoice", q.slug))
	}
	grace := 0
	if l.GraceDays != nil {
		grace = max(*l.GraceDays, 0)
	}
	wire, err := WireFor(ctx, in.Host, in.Subject)
	if err != nil {
		log.Error("invoice: no bank receives a wire on %q: %v", in.Host, err)
		return nil, false, coded(saleUnavailable, "wire_not_configured", "payment by bank transfer is not set up here, so nothing was ordered")
	}

	facts := "invoice:" + factsOf(sale)
	guard := in.IdempotencyKey
	if guard == "" {
		guard = windowKey(facts)
	}
	rec, replay, err := idempotencykey.Begin(db, invoiceScope+in.Subject, guard)
	switch {
	case err != nil:
		return nil, false, saleRefusal{saleUnavailable, "billing could not be reached; nothing was ordered, try again"}
	case rec.Facts != "" && rec.Facts != facts:
		return nil, false, coded(saleHeld, "key_reused", "this X-Idempotency-Key was used for another order: send a new key for a new order")
	case replay && rec.Status == idempotencykey.StatusCompleted && rec.Response != "":
		out, err := reread(ctx, db, org, rec.Response)
		return out, err == nil, err
	case replay:
		return nil, false, coded(saleHeld, "order_placing", "this order is already being placed")
	}
	rec.Facts = facts
	abandon := func() { _ = rec.Delete() }
	lease, err := holdSale(db, in.Subject)
	if err != nil {
		abandon()
		return nil, false, err
	}
	defer lease.Release()

	if held := billingSubscription(db, in.Subject, org.TestMode()); held != nil && subscriptionSlug(held) == q.slug {
		abandon()
		return nil, false, coded(saleHeld, "plan_held", fmt.Sprintf("this account already pays for the %q plan (subscription %s)", q.slug, held.Id()))
	}
	if open, err := openOrder(db, in.Subject, q.slug); err != nil {
		abandon()
		return nil, false, saleRefusal{saleUnavailable, "billing could not be reached; nothing was ordered, try again"}
	} else if open != nil {
		abandon()
		r := coded(saleHeld, "invoice_open", fmt.Sprintf("invoice %s for plan %q is open (%s): pay it, or ask us to void it, before ordering again", open.NumberStr, q.slug, open.Id()))
		r.Open = standing(org, open)
		return nil, false, r
	}
	net := in.Net
	if net {
		gone, err := defaulted(db, in.Subject)
		if err != nil {
			abandon()
			return nil, false, saleRefusal{saleUnavailable, "billing could not be reached; nothing was ordered, try again"}
		}
		net = !gone
	}

	now := time.Now()
	inv := billinginvoice.New(db)
	inv.UserId, inv.CustomerEmail, inv.Currency = in.Subject, in.Email, q.cur
	inv.PaymentMethod = string(processor.Wire)
	inv.LineItems = orderLines(q)
	inv.RecalculateSubtotal()
	if q.promoPercent > 0 {
		fee := inv.Subtotal - q.balance
		if d := fee - engine.Discounted(fee, q.promoPercent); d > 0 {
			inv.Discount, inv.DiscountName = d, q.promoName
		}
	}
	inv.DueDate = now.AddDate(0, 0, *l.InvoiceDays)
	if err := engine.Issue(db, inv); err != nil {
		abandon()
		return nil, false, err
	}
	o := &order{
		Plan: q.slug, Level: in.Level, Interval: q.plan.Interval, Quantity: q.qty, Lines: in.Lines,
		Seats: q.seats, Category: q.plan.Category, Balance: q.balance,
		Reference: invoiceReference(org.Name, inv.Number), Host: in.Host,
		Promo: q.promoPercent, PromoName: q.promoName, Net: net,
		VoidAt: inv.DueDate.AddDate(0, 0, grace),
	}
	o.keep(inv)
	if err := inv.Create(); err != nil {
		abandon()
		return nil, false, saleRefusal{saleUnavailable, "the invoice could not be written; nothing was ordered, try again"}
	}
	// What the order wrote is undone whole when a later step fails: the invoice is
	// voided, so a retry finds no open invoice in its way.
	undo := func() {
		if inv.MarkVoid() == nil {
			_ = inv.Update()
		}
		abandon()
	}
	link, err := invoicelink.New(org.Name, inv.Id(), org.TestMode())
	if err != nil {
		undo()
		return nil, false, saleRefusal{saleUnavailable, "the invoice could not be written; nothing was ordered, try again"}
	}
	var sub *subscription.Subscription
	if net {
		if sub, err = openPlan(ctx, db, org, inv, o); err != nil {
			undo()
			return nil, false, err
		}
		o.keep(inv)
		if err := inv.Update(); err != nil {
			log.Error("invoice %s: the plan it opened (%s) was not recorded on it: %v", inv.Id(), sub.Id(), err)
		}
	}
	wire.Reference, wire.Memo = o.Reference, o.Reference
	out := orderView(inv, o, &wirePay{wire: wire, voidAt: o.VoidAt}, sub)
	out.Link = link
	// The guard keeps the order without its link or its bank: a link is kept only as
	// its digest, and bank details are never stored. A retry reads them again.
	sealed := *out
	sealed.Wire, sealed.HowToPay, sealed.Link = nil, nil, ""
	if body, err := json.Marshal(&sealed); err == nil {
		_ = idempotencykey.Complete(rec, string(body))
	}
	return out, false, nil
}

// reread answers an order sent again under its key from the order the guard
// sealed: the invoice it raised as it stands now, a new link to it, and while it
// is open the bank read live, so its bank and its steps come from one read.
func reread(ctx context.Context, db *datastore.Datastore, org *organization.Organization, sealed string) (*InvoiceOrder, error) {
	var prev InvoiceOrder
	if json.Unmarshal([]byte(sealed), &prev) != nil || prev.InvoiceID == "" {
		return nil, saleRefusal{saleUnavailable, "the order could not be read; try again"}
	}
	inv, o, err := loadOrder(db, prev.InvoiceID)
	if err != nil {
		return nil, err
	}
	out, err := present(ctx, db, org, inv, o)
	if err != nil {
		return nil, err
	}
	if out.Link, err = invoicelink.New(org.Name, inv.Id(), org.TestMode()); err != nil {
		return nil, saleRefusal{saleUnavailable, "the link could not be written; try again"}
	}
	return out, nil
}

// present is an order's invoice as it stands: how to pay it while it is open, the
// bank read now, and the plan it opened.
func present(ctx context.Context, db *datastore.Datastore, org *organization.Organization, inv *billinginvoice.BillingInvoice, o *order) (*InvoiceOrder, error) {
	var pay *wirePay
	if inv.Status == billinginvoice.Open {
		var err error
		if pay, err = payOf(ctx, inv, o); err != nil {
			return nil, err
		}
	}
	sub, err := opened(db, inv.UserId, org.TestMode(), "invoice:"+inv.Id())
	if err != nil {
		return nil, saleRefusal{saleUnavailable, "the subscriptions could not be read; try again"}
	}
	return orderView(inv, o, pay, sub), nil
}

// standing is the open invoice an order was refused for, as its buyer reads it,
// with a new link to it: a link is kept only as its digest, so the one the order
// was answered with cannot be shown again, and every link minted for an invoice
// stays valid beside the others. A link that cannot be written leaves the view
// without one.
func standing(org *organization.Organization, inv *billinginvoice.BillingInvoice) *InvoiceOrder {
	o, ok := orderOf(inv)
	if !ok {
		return nil
	}
	view := orderView(inv, o, nil, nil)
	link, err := invoicelink.New(org.Name, inv.Id(), org.TestMode())
	if err != nil {
		log.Error("invoice %s: a link to it was not written: %v", inv.Id(), err)
		return view
	}
	view.Link = link
	return view
}

// defaulted reports whether subject let an order under net terms expire unpaid: an
// invoice the sweep voided (expiry) whose plan had opened before it was paid.
func defaulted(db *datastore.Datastore, subject string) (bool, error) {
	invs := make([]*billinginvoice.BillingInvoice, 0)
	if _, err := billinginvoice.Query(db).Filter("UserId=", subject).Filter("Status=", string(billinginvoice.Void)).GetAll(&invs); err != nil {
		return false, err
	}
	for _, inv := range invs {
		if o, ok := orderOf(inv); ok && o.Net && o.Reason == expiry {
			return true, nil
		}
	}
	return false, nil
}

// orderLines are an order's invoice lines: each seat type's seats at its price for
// the period, then the starting balance.
func orderLines(q *priced) []billinginvoice.LineItem {
	per := "year"
	if q.plan.Interval != types.Yearly {
		per = "month"
	}
	seatLine := func(plan, name string, n, cents int64) billinginvoice.LineItem {
		desc := fmt.Sprintf("%s: %d seat", name, n)
		if n != 1 {
			desc += "s"
		}
		desc += fmt.Sprintf(" × %s/seat/%s", fmtAmount(q.cur, cents), per)
		return billinginvoice.LineItem{
			Id: "li_plan_" + plan, Type: billinginvoice.LineSubscription, Description: desc,
			PlanId: plan, PlanName: name, Quantity: n, UnitPrice: cents, Amount: cents * n, Currency: q.cur,
		}
	}
	lines := []billinginvoice.LineItem{seatLine(q.slug, q.plan.Name, q.seatMult, int64(q.plan.Price))}
	for _, l := range q.lines {
		lines = append(lines, seatLine(l.Plan, l.Name, int64(l.Quantity), int64(l.Price)))
	}
	if q.balance > 0 {
		lines = append(lines, billinginvoice.LineItem{
			Id: "li_balance", Type: billinginvoice.LineOneOff, Description: "Initial usage balance",
			Amount: q.balance, Currency: q.cur,
		})
	}
	return lines
}

// invoiceReference is the reference an order's wire is sent with, naming the org
// and the invoice in the alphabet a bank's memo keeps ([A-Z0-9-]), then a check
// of the org's exact name and the number: "INV-ACME-IO-0042-K7Q2MZ3D". A bank
// drops separators, so "acme-io" and "acmeio", or "acme1" invoice 42 and "acme"
// invoice 10042, read alike without it; the check is 40 bits of SHA-256 over
// both, so two such references differ in it.
func invoiceReference(org string, number int) string {
	var b strings.Builder
	b.WriteString("INV-")
	for _, r := range strings.ToUpper(org) {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	sum := sha256.Sum256([]byte(org + "\n" + strconv.Itoa(number)))
	fmt.Fprintf(&b, "-%04d-%s", number, base32.StdEncoding.EncodeToString(sum[:5]))
	return b.String()
}

// unpaidText says what happens to an invoice that is not paid.
func unpaidText(voidAt time.Time) string {
	return fmt.Sprintf("If it is not paid by %s, the invoice is voided and nothing starts.", fmtDate(voidAt))
}

// orderView is how an order's invoice reads to its buyer. How to pay it (pay) is
// shown only while it is open; sub is the plan it opened, if any.
func orderView(inv *billinginvoice.BillingInvoice, o *order, pay *wirePay, sub *subscription.Subscription) *InvoiceOrder {
	out := &InvoiceOrder{
		InvoiceID: inv.Id(), Number: inv.NumberStr, Reference: o.Reference, Status: string(inv.Status),
		PlanID: o.Plan, Category: o.Category, Seats: o.Seats, AmountCents: inv.AmountDue,
		BalanceCents: o.Balance, Currency: string(inv.Currency), IssuedAt: inv.CreatedAt,
		DueAt: inv.DueDate, VoidAt: o.VoidAt, Email: inv.CustomerEmail, Replaced: o.Replaced, Payment: inv.PaymentRef, Net: o.Net,
		Brand: checkout.BrandSlugForHost(o.Host), Notice: InvoiceNotice, Unpaid: unpaidText(o.VoidAt),
	}
	if out.IssuedAt.IsZero() {
		out.IssuedAt = time.Now()
	}
	for _, li := range inv.LineItems {
		out.Lines = append(out.Lines, OrderLine{Description: li.Description, Quantity: li.Quantity, UnitCents: li.UnitPrice, AmountCents: li.Amount})
	}
	if !inv.PaidAt.IsZero() {
		at := inv.PaidAt
		out.PaidAt = &at
	}
	if !inv.VoidedAt.IsZero() {
		at := inv.VoidedAt
		out.VoidedAt = &at
	}
	if inv.Status == billinginvoice.Open && pay != nil {
		w := pay.wire
		out.Wire = &w
		out.HowToPay = pay.steps(fmtAmount(inv.Currency, inv.AmountDue), fmtDate(inv.DueDate))
	}
	if sub != nil {
		out.Active, out.SubscriptionID = sub.Status != subscription.Canceled, sub.Id()
	}
	return out
}

// openOrder is subject's open order invoice for plan slug, or nil.
func openOrder(db *datastore.Datastore, subject, slug string) (*billinginvoice.BillingInvoice, error) {
	invs := make([]*billinginvoice.BillingInvoice, 0)
	if _, err := billinginvoice.Query(db).Filter("UserId=", subject).Filter("Status=", string(billinginvoice.Open)).GetAll(&invs); err != nil {
		return nil, err
	}
	for _, inv := range invs {
		if o, ok := orderOf(inv); ok && o.Plan == slug {
			return inv, nil
		}
	}
	return nil, nil
}

// openPlan opens the plan an order's invoice is for — once: the subscription the
// invoice opened before (net terms, or a payment run again) is found by it, and
// stands again if a void ended it — at the order's seats, priced at the period
// ordered, the invoice its current one. The plan is collected outside commerce
// (External): its period was paid by this wire, so no renewal ever takes the
// next one from the org's balance or card. A paid plan the org held is ended as a
// card sale ends the one it replaces, its reload switched off, and named on the
// order (Replaced).
func openPlan(ctx context.Context, db *datastore.Datastore, org *organization.Organization, inv *billinginvoice.BillingInvoice, o *order) (*subscription.Subscription, error) {
	key := "invoice:" + inv.Id()
	sub, err := opened(db, inv.UserId, org.TestMode(), key)
	if err != nil {
		return nil, saleRefusal{saleUnavailable, "the subscriptions could not be read; try again"}
	}
	held := billingSubscription(db, inv.UserId, org.TestMode())
	switch {
	case sub != nil && sub.Status == subscription.Canceled:
		sub.Status, sub.Canceled, sub.CanceledAt, sub.Ended, sub.EndCancel = subscription.Active, false, time.Time{}, time.Time{}, false
	case sub == nil:
		p, err := resolveSubscriptionPlan(db, o.Plan)
		if err != nil {
			return nil, saleRefusal{saleUnavailable, "the plan could not be read; try again"}
		}
		if p, err = planAt(p, o.Level, o.Interval); err != nil {
			return nil, saleRefusal{saleRefused, err.Error()}
		}
		lines, err := linesAt(db, p, o.Lines, p.Interval, inv.Currency)
		if err != nil {
			return nil, saleRefusal{saleRefused, err.Error()}
		}
		p.TrialPeriodDays = 0
		if sub, err = createSubscription(db, p, &createSubscriptionRequest{
			UserId: inv.UserId, PlanId: o.Plan, DefaultPaymentMethod: string(processor.Wire),
			Quantity: o.Quantity, Metadata: map[string]interface{}{"source": "subscribe/invoice", "payment": key},
			Test: org.TestMode(), priced: lines,
		}); err != nil {
			return nil, err
		}
	}
	sub.Type = subscription.External
	sub.DiscountPercent, sub.DiscountName = o.Promo, o.PromoName
	sub.CurrentInvoiceId = inv.Id()
	if err := sub.Update(); err != nil {
		return nil, saleRefusal{saleUnavailable, "the subscription could not be written; try again"}
	}
	if held != nil && held.Id() != sub.Id() {
		was := subscription.New(db)
		if err := was.GetById(held.Id()); err == nil && was.Status != subscription.Canceled {
			_ = engine.CancelSubscription(was, false)
			if err := was.Update(); err != nil {
				log.Error("invoice %s: the plan it replaced (%s) was not ended: %v", inv.Id(), was.Id(), err)
			}
			if slug := subscriptionSlug(was); slug != o.Plan {
				disarm(ctx, org, inv.UserId, slug)
			}
		}
		o.Replaced = held.Id()
	}
	return sub, nil
}

// holdInvoice takes the lease an order's invoice is changed under, waiting up to
// paymentWait for its holder.
func holdInvoice(db *datastore.Datastore, id string) (*idempotencykey.Lease, error) {
	lease, held, err := hold(db, invoiceLease, id)
	switch {
	case err != nil:
		return nil, saleRefusal{saleUnavailable, "billing could not be reached; try again"}
	case !held:
		return nil, saleRefusal{saleHeld, "the invoice is being changed; try again in a moment"}
	}
	return lease, nil
}

// loadOrder reads an order's invoice by id from org's books.
func loadOrder(db *datastore.Datastore, id string) (*billinginvoice.BillingInvoice, *order, error) {
	inv := billinginvoice.New(db)
	if err := inv.GetById(id); err != nil {
		if errors.Is(err, datastore.ErrNoSuchEntity) {
			return nil, nil, saleRefusal{saleMissing, "invoice not found"}
		}
		return nil, nil, saleRefusal{saleUnavailable, "the invoice could not be read; try again"}
	}
	o, ok := orderOf(inv)
	if !ok {
		return nil, nil, saleRefusal{saleRefused, fmt.Sprintf("invoice %s is not an order paid by wire", id)}
	}
	return inv, o, nil
}

// SettleIn is a wire that paid an order's invoice, as the bank states it: its
// receipt (the bank's reference for the payment), the amount received and when
// (zero is now). An amount that is not the invoice's is taken only when
// Difference says which way it misses: "short" or "over".
type SettleIn struct {
	InvoiceID   string
	Receipt     string
	AmountCents int64
	ReceivedAt  time.Time
	Difference  string
}

// Settled is a paid order: its invoice, the plan it opened, and the starting
// balance credited (BalanceID its receipt; nothing when the payment bought none).
// Reopened says a wire received before the invoice was voided re-opened it.
// Replayed says this answers a settlement already made in full: nothing moved
// now, and the buyer was told then.
type Settled struct {
	Order          *InvoiceOrder `json:"order"`
	Payment        string        `json:"payment"`
	SubscriptionID string        `json:"subscriptionId"`
	ReceivedCents  int64         `json:"receivedCents"`
	ShortCents     int64         `json:"shortCents,omitempty"`
	OverCents      int64         `json:"overCents,omitempty"`
	BalanceCents   int64         `json:"balanceCents,omitempty"`
	BalanceID      string        `json:"balanceId,omitempty"`
	Reopened       bool          `json:"reopened,omitempty"`
	Replayed       bool          `json:"replayed,omitempty"`
}

// WireKey is the one name a wire's bank receipt is kept under, whatever the bank
// or the person typing it did to it: "wire:" and the receipt's ASCII letters and
// digits, upper-cased, with any "wire:" it already carried dropped.
// "FED-2026-0001", "fed 2026 0001" and "wire:FED20260001" are one key; a receipt
// with no letter or digit is "". Cloud names a wire it records as credit the same
// way (apps/admin/core KeyOf of "wire:"+receipt), so one wire is one key on both
// rails.
func WireKey(receipt string) string {
	p := strings.TrimSpace(receipt)
	for len(p) >= len("wire:") && strings.EqualFold(p[:len("wire:")], "wire:") {
		p = strings.TrimSpace(p[len("wire:"):])
	}
	var b strings.Builder
	for _, r := range p {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r - 'a' + 'A')
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return ""
	}
	return string(processor.Wire) + ":" + b.String()
}

// SettleInvoice marks an order's invoice paid by a wire, in one step that is done
// once however often it is asked, under the invoice's lease: the payment is
// recorded under its receipt's key (WireKey; paymentorg, as a card sale's payment
// is) and held for this invoice alone, the plan opens (or the one net terms
// opened is kept), the invoice is paid for the plan's first period, and the
// starting balance is credited through the payment's record and lease into both
// books (endow). A short payment is short of the balance first and never of the
// seats. The same receipt again answers the same; a receipt that settled another
// payment, a paid invoice, a received date after tomorrow or before the invoice
// was issued, are refused. A void invoice is re-opened and paid only when the bank
// received the wire before it was voided.
func SettleInvoice(ctx context.Context, org *organization.Organization, in SettleIn) (*Settled, error) {
	payment := WireKey(in.Receipt)
	now := time.Now()
	switch {
	case org == nil:
		return nil, errors.New("invoice: no organization")
	case payment == "":
		return nil, saleRefusal{saleRefused, "receipt is required: the bank's reference for the payment"}
	case in.AmountCents <= 0:
		return nil, saleRefusal{saleRefused, "amountCents is required: what the bank received"}
	case in.ReceivedAt.After(now.Add(receivedSkew)):
		return nil, saleRefusal{saleRefused, fmt.Sprintf("receivedAt %s is in the future: send the date the bank received the wire", fmtDate(in.ReceivedAt))}
	}
	db := datastore.New(org.Namespaced(ctx))
	lease, err := holdInvoice(db, in.InvoiceID)
	if err != nil {
		return nil, err
	}
	defer lease.Release()
	inv, o, err := loadOrder(db, in.InvoiceID)
	if err != nil {
		return nil, err
	}
	if issued := inv.CreatedAt.UTC().Truncate(24 * time.Hour); !in.ReceivedAt.IsZero() && in.ReceivedAt.Before(issued) {
		return nil, saleRefusal{saleRefused, fmt.Sprintf("receivedAt %s is before invoice %s was issued (%s)", fmtDate(in.ReceivedAt), inv.NumberStr, fmtDate(inv.CreatedAt))}
	}
	switch inv.Status {
	case billinginvoice.Paid:
		if inv.PaymentRef != payment {
			return nil, saleRefusal{saleHeld, fmt.Sprintf("invoice %s is already paid by %s", inv.NumberStr, inv.PaymentRef)}
		}
		// Settled in full before: its credit was decided and has landed.
		po, found, err := paymentorg.Get(payment)
		if err != nil {
			return nil, saleRefusal{saleUnavailable, "billing could not be reached; try again"}
		}
		out, err := paid(ctx, db, org, inv, o, payment)
		if out != nil {
			out.Replayed = found && po.Credited && (po.Credit == 0 || po.Deposit != "")
		}
		return out, err
	case billinginvoice.Void:
		if in.ReceivedAt.IsZero() || !in.ReceivedAt.Before(inv.VoidedAt) {
			return nil, saleRefusal{saleHeld, fmt.Sprintf("invoice %s was voided on %s, so it cannot be paid: a wire the bank received before then re-opens it (send receivedAt)",
				inv.NumberStr, fmtDate(inv.VoidedAt))}
		}
		voided := inv.VoidedAt
		inv.Status, inv.VoidedAt = billinginvoice.Open, time.Time{}
		o.Voided, o.Reopened = &voided, &now
	case billinginvoice.Open:
	default:
		return nil, saleRefusal{saleHeld, fmt.Sprintf("invoice %s is %s, so it cannot be paid", inv.NumberStr, inv.Status)}
	}
	diff := in.AmountCents - inv.AmountDue
	switch {
	case diff < 0 && in.Difference != "short":
		return nil, saleRefusal{saleRefused, fmt.Sprintf("the invoice is %s and %s was received: send difference \"short\" to take a short payment",
			fmtAmount(inv.Currency, inv.AmountDue), fmtAmount(inv.Currency, in.AmountCents))}
	case diff > 0 && in.Difference != "over":
		return nil, saleRefusal{saleRefused, fmt.Sprintf("the invoice is %s and %s was received: send difference \"over\" to take an overpayment",
			fmtAmount(inv.Currency, inv.AmountDue), fmtAmount(inv.Currency, in.AmountCents))}
	case diff == 0 && in.Difference != "":
		return nil, saleRefusal{saleRefused, "the amount received is the invoice's: send no difference"}
	}
	if seats := inv.AmountDue - o.Balance; in.AmountCents < seats {
		return nil, saleRefusal{saleRefused, fmt.Sprintf("a short payment is short of the starting balance only: the seats are %s and %s was received",
			fmtAmount(inv.Currency, seats), fmtAmount(inv.Currency, in.AmountCents))}
	}
	// A receipt names one payment, and the payment one invoice: it is recorded,
	// then held for this invoice in one swap, so a receipt that settled (or is
	// settling) another invoice, here or in another org, is refused.
	if err := paymentorg.Open(payment, org.Name, inv.UserId, "", string(processor.Wire), max(o.Balance+min(diff, 0), 0), org.TestMode()); err != nil {
		return nil, saleRefusal{saleUnavailable, "the payment could not be recorded; try again"}
	}
	mine := false
	if _, _, err := paymentorg.Change(payment, func(r *paymentorg.PaymentOrg) bool {
		mine = strings.EqualFold(r.Org, org.Name) && (r.Invoice == "" || r.Invoice == inv.Id())
		if !mine || r.Invoice == inv.Id() {
			return false
		}
		r.Invoice = inv.Id()
		return true
	}); err != nil {
		return nil, saleRefusal{saleUnavailable, "the payment could not be recorded; try again"}
	}
	if !mine {
		return nil, saleRefusal{saleHeld, fmt.Sprintf("receipt %s already settled another payment", strings.TrimPrefix(payment, "wire:"))}
	}
	sub, err := openPlan(ctx, db, org, inv, o)
	if err != nil {
		return nil, err
	}
	inv.SubscriptionId, inv.PeriodStart, inv.PeriodEnd = sub.Id(), sub.PeriodStart, sub.PeriodEnd
	for i := range inv.LineItems {
		inv.LineItems[i].PeriodStart, inv.LineItems[i].PeriodEnd = sub.PeriodStart, sub.PeriodEnd
	}
	if err := inv.MarkPaid(string(processor.Wire), payment); err != nil {
		return nil, saleRefusal{saleHeld, err.Error()}
	}
	inv.AmountPaid = in.AmountCents
	if !in.ReceivedAt.IsZero() {
		inv.PaidAt = in.ReceivedAt
	}
	o.Received = in.AmountCents
	o.keep(inv)
	if err := inv.Update(); err != nil {
		return nil, saleRefusal{saleUnavailable, "the invoice could not be written; try again"}
	}
	return paid(ctx, db, org, inv, o, payment)
}

// paid credits a paid order's starting balance (endow) and answers it. It is
// the last step of SettleInvoice and all of its retry.
func paid(ctx context.Context, db *datastore.Datastore, org *organization.Organization, inv *billinginvoice.BillingInvoice, o *order, payment string) (*Settled, error) {
	po, err := endow(ctx, nil, org, db, inv.Currency, payment)
	if err != nil {
		return nil, err
	}
	sub, err := opened(db, inv.UserId, org.TestMode(), "invoice:"+inv.Id())
	if err != nil || sub == nil {
		return nil, saleRefusal{saleUnavailable, "the subscription could not be read; try again"}
	}
	out := &Settled{
		Order: orderView(inv, o, nil, sub), Payment: payment, SubscriptionID: sub.Id(), ReceivedCents: o.Received,
		BalanceCents: po.Credit, BalanceID: po.Deposit, Reopened: o.Reopened != nil,
	}
	if d := o.Received - inv.AmountDue; d < 0 {
		out.ShortCents = -d
	} else {
		out.OverCents = d
	}
	return out, nil
}

// VoidOrder voids an order's open invoice, activating nothing — or, under net
// terms, ending the plan the order opened — and keeps why. A paid invoice is
// refunded, never voided; a void one answers as it is.
func VoidOrder(ctx context.Context, org *organization.Organization, id, reason string) (*InvoiceOrder, error) {
	if org == nil {
		return nil, errors.New("invoice: no organization")
	}
	db := datastore.New(org.Namespaced(ctx))
	lease, err := holdInvoice(db, id)
	if err != nil {
		return nil, err
	}
	defer lease.Release()
	inv, o, err := loadOrder(db, id)
	if err != nil {
		return nil, err
	}
	switch inv.Status {
	case billinginvoice.Void:
		return orderView(inv, o, nil, nil), nil
	case billinginvoice.Paid:
		return nil, saleRefusal{saleHeld, fmt.Sprintf("invoice %s is paid: refund it instead", inv.NumberStr)}
	}
	if err := inv.MarkVoid(); err != nil {
		return nil, saleRefusal{saleHeld, err.Error()}
	}
	o.Reason = strings.TrimSpace(reason)
	o.keep(inv)
	if err := inv.Update(); err != nil {
		return nil, saleRefusal{saleUnavailable, "the invoice could not be written; try again"}
	}
	if sub, err := opened(db, inv.UserId, org.TestMode(), "invoice:"+inv.Id()); err == nil && sub != nil && sub.Status != subscription.Canceled {
		if err := engine.CancelSubscription(sub, false); err == nil {
			_ = sub.Update()
		}
	}
	return orderView(inv, o, nil, nil), nil
}

// RefundOrder gives back amount of a paid order's wire, under ref (the bank's
// reference for the refund): as a card sale's refund is (takeOrder), the starting
// balance first — netted from a credit not yet decided, taken from the wallet
// after — and the rest recorded on the invoice; a refund of the whole ends the
// period it paid (periodPaid). It never gives back more than was paid, the
// balance's part counted (refunded), and the same ref again changes nothing.
func RefundOrder(ctx context.Context, org *organization.Organization, id string, amount int64, ref string) (*InvoiceOrder, error) {
	ref = strings.TrimSpace(ref)
	switch {
	case org == nil:
		return nil, errors.New("invoice: no organization")
	case ref == "":
		return nil, saleRefusal{saleRefused, "reference is required: the bank's reference for the refund"}
	case amount <= 0:
		return nil, saleRefusal{saleRefused, "amountCents is required"}
	}
	db := datastore.New(org.Namespaced(ctx))
	inv, o, err := loadOrder(db, id)
	if err != nil {
		return nil, err
	}
	if inv.Status != billinginvoice.Paid {
		return nil, saleRefusal{saleHeld, fmt.Sprintf("invoice %s is %s: only a paid invoice is refunded", inv.NumberStr, inv.Status)}
	}
	lease, held, err := holdPayment(db, inv.PaymentRef)
	switch {
	case err != nil:
		return nil, saleRefusal{saleUnavailable, "billing could not be reached; try again"}
	case !held:
		return nil, saleRefusal{saleHeld, "the payment is being changed; try again in a moment"}
	}
	defer lease.Release()
	where, found, err := paymentorg.Get(inv.PaymentRef)
	if err != nil || !found || inv.GetById(inv.Id()) != nil {
		return nil, saleRefusal{saleUnavailable, "the payment could not be read; try again"}
	}
	// Bounded under the payment's lease, so two refunds read one another.
	if left := inv.AmountPaid - refunded(inv, where); where.Took("refund:"+ref) == nil && amount > left {
		return nil, saleRefusal{saleRefused, fmt.Sprintf("invoice %s holds %s to refund", inv.NumberStr, fmtAmount(inv.Currency, left))}
	}
	if err := takeOrder(ctx, db, where, "refund.wire", "refund:"+ref, ref, inv.PaymentRef, currency.Cents(amount), inv.Currency, "", 0, time.Time{}); err != nil {
		return nil, saleRefusal{saleUnavailable, err.Error()}
	}
	if err := inv.GetById(inv.Id()); err != nil {
		return nil, saleRefusal{saleUnavailable, "the invoice could not be read; try again"}
	}
	return orderView(inv, o, nil, nil), nil
}

// refunded is what refunds gave back of an order's wire. Its invoice holds the
// starting balance beside the seats, and a refund takes that balance first, on
// the payment's record (takeOrder), recording only the rest on the invoice; so
// both are counted, never past what was paid.
func refunded(inv *billinginvoice.BillingInvoice, po *paymentorg.PaymentOrg) int64 {
	return min(inv.Refunded()+po.Refunded(), inv.AmountPaid)
}

// Expired is an order's invoice the sweep voided: unpaid past its due date and
// grace.
type Expired struct {
	Org   string        `json:"org"`
	Order *InvoiceOrder `json:"order"`
}

// ExpireInvoices voids every order invoice in every org unpaid past its due date
// and grace at now (VoidOrder), and answers them, so the caller tells their
// buyers. One that cannot be voided is logged and left for the next sweep.
func ExpireInvoices(ctx context.Context, now time.Time) ([]Expired, error) {
	orgs := make([]*organization.Organization, 0)
	if _, err := organization.Query(datastore.New(ctx)).GetAll(&orgs); err != nil {
		return nil, err
	}
	out := make([]Expired, 0)
	for _, org := range orgs {
		db := datastore.New(org.Namespaced(ctx))
		invs := make([]*billinginvoice.BillingInvoice, 0)
		if _, err := billinginvoice.Query(db).Filter("Status=", string(billinginvoice.Open)).Filter("PaymentMethod=", string(processor.Wire)).GetAll(&invs); err != nil {
			log.Error("invoice expiry: the open invoices of %s could not be read: %v", org.Name, err)
			continue
		}
		for _, inv := range invs {
			o, ok := orderOf(inv)
			if !ok || o.VoidAt.IsZero() || now.Before(o.VoidAt) {
				continue
			}
			view, err := VoidOrder(ctx, org, inv.Id(), expiry)
			if err != nil {
				log.Error("invoice expiry: invoice %s of %s was not voided: %v", inv.Id(), org.Name, err)
				continue
			}
			out = append(out, Expired{Org: org.Name, Order: view})
		}
	}
	return out, nil
}

// Order is an order's invoice in org's books as staff read it: as its buyer reads
// it, with how to pay it while it is open (the bank read now) and the plan it
// opened, and the payment that paid it. It mints no link.
func Order(ctx context.Context, org *organization.Organization, id string) (*InvoiceOrder, error) {
	if org == nil {
		return nil, errors.New("invoice: no organization")
	}
	db := datastore.New(org.Namespaced(ctx))
	inv, o, err := loadOrder(db, strings.TrimSpace(id))
	if err != nil {
		return nil, err
	}
	return present(ctx, db, org, inv, o)
}

// Orders is org's order invoices, newest first: those of status when it names one
// ("open", "paid", "void"), else every one. A list reads no bank and no plan; Order
// reads one invoice whole.
func Orders(ctx context.Context, org *organization.Organization, status string) ([]*InvoiceOrder, error) {
	if org == nil {
		return nil, errors.New("invoice: no organization")
	}
	q := billinginvoice.Query(datastore.New(org.Namespaced(ctx))).Filter("PaymentMethod=", string(processor.Wire))
	if s := strings.ToLower(strings.TrimSpace(status)); s != "" {
		q = q.Filter("Status=", s)
	}
	invs := make([]*billinginvoice.BillingInvoice, 0)
	if _, err := q.GetAll(&invs); err != nil {
		return nil, saleRefusal{saleUnavailable, "the invoices could not be read; try again"}
	}
	out := make([]*InvoiceOrder, 0, len(invs))
	for _, inv := range invs {
		if o, ok := orderOf(inv); ok {
			out = append(out, orderView(inv, o, nil, nil))
		}
	}
	slices.SortFunc(out, func(a, b *InvoiceOrder) int { return b.IssuedAt.Compare(a.IssuedAt) })
	return out, nil
}

// InvoiceLink mints another link to an order's invoice in org's books, for a
// reader of the org who lost the first.
func InvoiceLink(ctx context.Context, org *organization.Organization, id string) (string, error) {
	if org == nil {
		return "", errors.New("invoice: no organization")
	}
	inv, _, err := loadOrder(datastore.New(org.Namespaced(ctx)), id)
	if err != nil {
		return "", err
	}
	link, err := invoicelink.New(org.Name, inv.Id(), org.TestMode())
	if err != nil {
		return "", saleRefusal{saleUnavailable, "the link could not be written; try again"}
	}
	return link, nil
}

// Hosted is an order's invoice as anyone holding its link reads it: the order —
// what it is for, when it is due, and while it is open how to pay it — and who
// bills whom. The order is nested, as cloud's client.HostedInvoice and the pay
// page read it. It never carries the buyer's address or the receipt that paid it.
type Hosted struct {
	Order         *InvoiceOrder `json:"order"`
	Issuer        string        `json:"issuer"`
	IssuerAddress string        `json:"issuerAddress,omitempty"`
	BillTo        string        `json:"billTo"`
}

// HostedInvoice reads the order invoice a link shows, on the brand it is read on:
// a link is read only on the brand that issued its invoice. An unknown link, or
// one read on another brand, is a miss, never a hint that one exists.
func HostedInvoice(ctx context.Context, link, brand string) (*Hosted, error) {
	org, inv, o, err := linked(ctx, link, brand)
	if err != nil {
		return nil, err
	}
	var pay *wirePay
	if inv.Status == billinginvoice.Open {
		if pay, err = payOf(ctx, inv, o); err != nil {
			return nil, err
		}
	}
	view := orderView(inv, o, pay, nil)
	view.Email, view.Payment = "", ""
	is := issuerFor(view.Brand)
	return &Hosted{Order: view, Issuer: is.legal, IssuerAddress: is.address, BillTo: billedName(org, inv)}, nil
}

// HostedInvoicePDF renders the order invoice a link shows on the brand it is read
// on, with how to pay it while it is open. It bills the org by name and prints no
// buyer's address: the address is the buyer's sign-in, never one they chose to
// print, and a link travels.
func HostedInvoicePDF(ctx context.Context, link, brand string) (*Document, error) {
	org, inv, o, err := linked(ctx, link, brand)
	if err != nil {
		return nil, err
	}
	var pay *wirePay
	if inv.Status == billinginvoice.Open {
		if pay, err = payOf(ctx, inv, o); err != nil {
			return nil, err
		}
	}
	inv.CustomerEmail = ""
	return &Document{Filename: inv.NumberStr, Body: renderInvoicePDF(inv, billedName(org, inv), checkout.BrandSlugForHost(o.Host), pay)}, nil
}

// linked resolves a link to the order invoice it shows, when brand issued it.
func linked(ctx context.Context, link, brand string) (*organization.Organization, *billinginvoice.BillingInvoice, *order, error) {
	l, found, err := invoicelink.Find(link)
	switch {
	case err != nil:
		return nil, nil, nil, saleRefusal{saleUnavailable, "the invoice could not be read; try again"}
	case !found:
		return nil, nil, nil, saleRefusal{saleMissing, "invoice not found"}
	}
	org, err := orgNamed(ctx, l.Org)
	if err != nil {
		return nil, nil, nil, saleRefusal{saleMissing, "invoice not found"}
	}
	inv, o, err := loadOrder(datastore.New(org.Namespaced(ctx)), l.Invoice)
	if err != nil {
		return nil, nil, nil, err
	}
	if !strings.EqualFold(checkout.BrandSlugForHost(o.Host), strings.TrimSpace(brand)) {
		return nil, nil, nil, saleRefusal{saleMissing, "invoice not found"}
	}
	return org, inv, o, nil
}

// billedName is who an invoice bills: the org's name as it states it.
func billedName(org *organization.Organization, inv *billinginvoice.BillingInvoice) string {
	if org.FullName != "" {
		return org.FullName
	}
	return org.Name
}

// wirePay is how an open order invoice is paid: the receiving bank, the
// reference, and when it is voided if unpaid.
type wirePay struct {
	wire   WireInstructions
	voidAt time.Time
}

// payOf reads the bank an order's wire is sent to, now, with its reference.
func payOf(ctx context.Context, inv *billinginvoice.BillingInvoice, o *order) (*wirePay, error) {
	w, err := WireFor(ctx, o.Host, inv.UserId)
	if err != nil {
		log.Error("invoice %s: no bank receives a wire on %q: %v", inv.Id(), o.Host, err)
		return nil, saleRefusal{saleUnavailable, "payment by bank transfer is not set up here"}
	}
	w.Reference, w.Memo = o.Reference, o.Reference
	return &wirePay{wire: w, voidAt: o.VoidAt}, nil
}

// steps are how to pay, one field to a line so each copies on its own: the amount
// and the date, the reference to send exactly, then the bank, then what starts
// when and what happens if it is not paid.
func (p *wirePay) steps(amount, due string) []string {
	w := p.wire
	out := []string{
		"Amount due: " + amount,
		"Due date: " + due,
		"Reference: " + w.Reference,
		"Include this reference exactly, so we can match your payment.",
		"Bank: " + w.BankName,
	}
	if w.BankAddress != "" {
		out = append(out, "Bank address: "+w.BankAddress)
	}
	out = append(out, "Account name: "+w.AccountName)
	if w.AccountNumber != "" {
		out = append(out, "Account number: "+w.AccountNumber)
	}
	if w.RoutingNumber != "" {
		out = append(out, "Routing number: "+w.RoutingNumber)
	}
	if w.IBAN != "" {
		out = append(out, "IBAN: "+w.IBAN)
	}
	if w.SwiftCode != "" {
		out = append(out, "SWIFT: "+w.SwiftCode)
	}
	return append(out, InvoiceNotice, unpaidText(p.voidAt))
}

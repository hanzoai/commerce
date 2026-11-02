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
	"encoding/json"
	"errors"
	"fmt"
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
// it is for, when it is due and when it is voided if unpaid, the bank details and
// the reference to pay it with while it is open, and the link it is shown by
// (only on the answer to the order itself). Active and SubscriptionID say the
// plan is open: once paid, or at once under net terms.
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
	Link           string            `json:"link,omitempty"`
	Active         bool              `json:"active"`
	SubscriptionID string            `json:"subscriptionId,omitempty"`
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

// OrderInvoice raises the invoice for in's order and activates nothing: an OPEN
// invoice of the seats at the row's price and the starting balance, due the row's
// invoiceDays from now, with the link it is shown by and the bank details to pay
// it with. Under net terms the plan opens now. It answers the order, and whether
// it is the sealed answer to an earlier identical request (a retry under the same
// key), which is answered byte for byte as it was.
//
// It refuses what a card sale refuses (price), and a plan whose row states no
// invoiceDays; a brand with no bank to receive the wire is unavailable. One order
// of a subject is in flight at a time (holdSale), an org that already pays for the
// plan is told so, and an order for a plan with an invoice still open names that
// invoice rather than raise a second.
func OrderInvoice(ctx context.Context, org *organization.Organization, in OrderIn) (*InvoiceOrder, json.RawMessage, error) {
	if org == nil {
		return nil, nil, errors.New("invoice: no organization")
	}
	if in.Subject == "" {
		return nil, nil, errors.New("invoice: no subject")
	}
	db := datastore.New(org.Namespaced(ctx))
	planID := strings.TrimSpace(in.PlanID)
	if planID == "" {
		return nil, nil, saleRefusal{saleRefused, "planId is required"}
	}
	if in.StoreID != "" {
		if err := storemodel.New(db).GetById(in.StoreID); err != nil {
			return nil, nil, saleRefusal{saleMissing, "store not found"}
		}
	}
	sale := SubscribeIn{
		Subject: in.Subject, PlanID: planID, Quantity: in.Quantity, Lines: in.Lines, Level: in.Level,
		Interval: in.Interval, BalanceCents: in.BalanceCents, StoreID: in.StoreID, Promo: in.Promo,
	}
	q, err := price(db, org, sale, planID, "")
	if err != nil {
		return nil, nil, err
	}
	l := q.plan.Limits
	if l == nil || l.InvoiceDays == nil || *l.InvoiceDays <= 0 {
		return nil, nil, saleRefusal{saleRefused, fmt.Sprintf("plan %q is not sold by invoice", q.slug)}
	}
	grace := 0
	if l.GraceDays != nil {
		grace = max(*l.GraceDays, 0)
	}
	wire, err := WireFor(ctx, in.Host, in.Subject)
	if err != nil {
		log.Error("invoice: no bank receives a wire on %q: %v", in.Host, err)
		return nil, nil, saleRefusal{saleUnavailable, "payment by bank transfer is not set up here, so nothing was ordered"}
	}

	facts := "invoice:" + factsOf(sale)
	guard := in.IdempotencyKey
	if guard == "" {
		guard = windowKey(facts)
	}
	rec, replay, err := idempotencykey.Begin(db, invoiceScope+in.Subject, guard)
	switch {
	case err != nil:
		return nil, nil, saleRefusal{saleUnavailable, "billing could not be reached; nothing was ordered, try again"}
	case rec.Facts != "" && rec.Facts != facts:
		return nil, nil, saleRefusal{saleHeld, "this X-Idempotency-Key was used for another order: send a new key for a new order"}
	case replay && rec.Status == idempotencykey.StatusCompleted && rec.Response != "":
		return nil, json.RawMessage(rec.Response), nil
	case replay:
		return nil, nil, saleRefusal{saleHeld, "this order is already being placed"}
	}
	rec.Facts = facts
	abandon := func() { _ = rec.Delete() }
	lease, err := holdSale(db, in.Subject)
	if err != nil {
		abandon()
		return nil, nil, err
	}
	defer lease.Release()

	if held := billingSubscription(db, in.Subject, org.TestMode()); held != nil && subscriptionSlug(held) == q.slug {
		abandon()
		return nil, nil, saleRefusal{saleHeld, fmt.Sprintf("this account already pays for the %q plan (subscription %s)", q.slug, held.Id())}
	}
	if open, err := openOrder(db, in.Subject, q.slug); err != nil {
		abandon()
		return nil, nil, saleRefusal{saleUnavailable, "billing could not be reached; nothing was ordered, try again"}
	} else if open != nil {
		abandon()
		return nil, nil, saleRefusal{saleHeld, fmt.Sprintf("invoice %s for plan %q is open (%s): pay it, or ask us to void it, before ordering again", open.NumberStr, q.slug, open.Id())}
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
		return nil, nil, err
	}
	o := &order{
		Plan: q.slug, Level: in.Level, Interval: q.plan.Interval, Quantity: q.qty, Lines: in.Lines,
		Seats: q.seats, Category: q.plan.Category, Balance: q.balance,
		Reference: invoiceReference(org.Name, inv.Number), Host: in.Host,
		Promo: q.promoPercent, PromoName: q.promoName, Net: in.Net,
		VoidAt: inv.DueDate.AddDate(0, 0, grace),
	}
	o.keep(inv)
	if err := inv.Create(); err != nil {
		abandon()
		return nil, nil, saleRefusal{saleUnavailable, "the invoice could not be written; nothing was ordered, try again"}
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
		return nil, nil, saleRefusal{saleUnavailable, "the invoice could not be written; nothing was ordered, try again"}
	}
	var sub *subscription.Subscription
	if in.Net {
		if sub, err = openPlan(db, org, inv, o); err != nil {
			undo()
			return nil, nil, err
		}
	}
	wire.Reference, wire.Memo = o.Reference, o.Reference
	out := orderView(inv, o, &wire, sub)
	out.Link = link
	if body, err := json.Marshal(out); err == nil {
		_ = idempotencykey.Complete(rec, string(body))
	}
	return out, nil, nil
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
// and the invoice in the alphabet a bank's memo keeps ([A-Z0-9-]):
// "INV-ACME-0042".
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
	fmt.Fprintf(&b, "-%04d", number)
	return b.String()
}

// unpaidText says what happens to an invoice that is not paid.
func unpaidText(voidAt time.Time) string {
	return fmt.Sprintf("If it is not paid by %s, the invoice is voided and nothing starts.", fmtDate(voidAt))
}

// orderView is how an order's invoice reads to its buyer. wire is shown only while
// it is open; sub is the plan it opened, if any.
func orderView(inv *billinginvoice.BillingInvoice, o *order, wire *WireInstructions, sub *subscription.Subscription) *InvoiceOrder {
	out := &InvoiceOrder{
		InvoiceID: inv.Id(), Number: inv.NumberStr, Reference: o.Reference, Status: string(inv.Status),
		PlanID: o.Plan, Category: o.Category, Seats: o.Seats, AmountCents: inv.AmountDue,
		BalanceCents: o.Balance, Currency: string(inv.Currency), IssuedAt: inv.CreatedAt,
		DueAt: inv.DueDate, VoidAt: o.VoidAt, Email: inv.CustomerEmail,
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
	if inv.Status == billinginvoice.Open {
		out.Wire = wire
	}
	if sub != nil {
		out.Active, out.SubscriptionID = true, sub.Id()
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
// invoice opened before (net terms, or a payment run again) is found by it — at
// the order's seats, priced at the period ordered, the invoice its current one.
func openPlan(db *datastore.Datastore, org *organization.Organization, inv *billinginvoice.BillingInvoice, o *order) (*subscription.Subscription, error) {
	key := "invoice:" + inv.Id()
	sub, err := opened(db, inv.UserId, org.TestMode(), key)
	if err != nil {
		return nil, saleRefusal{saleUnavailable, "the subscriptions could not be read; try again"}
	}
	if sub == nil {
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
	sub.DiscountPercent, sub.DiscountName = o.Promo, o.PromoName
	sub.CurrentInvoiceId = inv.Id()
	if err := sub.Update(); err != nil {
		return nil, saleRefusal{saleUnavailable, "the subscription could not be written; try again"}
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
// receipt (the bank's reference for the payment), the amount received and when.
// An amount that is not the invoice's is taken only when Difference says which way
// it misses: "short" or "over".
type SettleIn struct {
	InvoiceID   string
	Receipt     string
	AmountCents int64
	ReceivedAt  time.Time
	Difference  string
}

// Settled is a paid order: its invoice, the plan it opened, and the starting
// balance credited (BalanceID its receipt; nothing when the payment bought none).
type Settled struct {
	Order          *InvoiceOrder `json:"order"`
	Payment        string        `json:"payment"`
	SubscriptionID string        `json:"subscriptionId"`
	ReceivedCents  int64         `json:"receivedCents"`
	ShortCents     int64         `json:"shortCents,omitempty"`
	OverCents      int64         `json:"overCents,omitempty"`
	BalanceCents   int64         `json:"balanceCents,omitempty"`
	BalanceID      string        `json:"balanceId,omitempty"`
}

// SettleInvoice marks an order's invoice paid by a wire, in one step that is done
// once however often it is asked, under the invoice's lease: the payment is
// recorded as \"wire:<receipt>\" (paymentorg, as a card sale's payment is), the
// plan opens (or the one net terms opened is kept), the invoice is paid for the
// plan's first period, and the starting balance is credited through the payment's
// record and lease into both books (endow). A short payment is short of the
// balance first. The same receipt again answers the same; a receipt that settled
// another payment, a paid or void invoice, are refused.
func SettleInvoice(ctx context.Context, org *organization.Organization, in SettleIn) (*Settled, error) {
	receipt := strings.TrimSpace(in.Receipt)
	switch {
	case org == nil:
		return nil, errors.New("invoice: no organization")
	case receipt == "":
		return nil, saleRefusal{saleRefused, "receipt is required: the bank's reference for the payment"}
	case in.AmountCents <= 0:
		return nil, saleRefusal{saleRefused, "amountCents is required: what the bank received"}
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
	payment := string(processor.Wire) + ":" + receipt
	switch inv.Status {
	case billinginvoice.Paid:
		if inv.PaymentRef != payment {
			return nil, saleRefusal{saleHeld, fmt.Sprintf("invoice %s is already paid by %s", inv.NumberStr, inv.PaymentRef)}
		}
		return paid(ctx, db, org, inv, o, payment)
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
	// A receipt names one payment: one that settled another is refused.
	if po, found, err := paymentorg.Get(payment); err != nil {
		return nil, saleRefusal{saleUnavailable, "billing could not be reached; try again"}
	} else if found && (po.Org != org.Name || (po.Invoice != "" && po.Invoice != inv.Id())) {
		return nil, saleRefusal{saleHeld, fmt.Sprintf("receipt %s already settled another payment", receipt)}
	}
	if err := paymentorg.Open(payment, org.Name, inv.UserId, "", string(processor.Wire), max(o.Balance+min(diff, 0), 0), org.TestMode()); err != nil {
		return nil, saleRefusal{saleUnavailable, "the payment could not be recorded; try again"}
	}
	sub, err := openPlan(db, org, inv, o)
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
	if _, _, err := paymentorg.Change(payment, func(r *paymentorg.PaymentOrg) bool {
		if r.Invoice == inv.Id() {
			return false
		}
		r.Invoice = inv.Id()
		return true
	}); err != nil {
		return nil, saleRefusal{saleUnavailable, "the payment could not be recorded; try again"}
	}
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
		BalanceCents: po.Credit, BalanceID: po.Deposit,
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
// period it paid. The same ref again changes nothing.
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
	if amount > inv.AmountPaid-inv.Refunded() {
		return nil, saleRefusal{saleRefused, fmt.Sprintf("invoice %s holds %s to refund", inv.NumberStr, fmtAmount(inv.Currency, inv.AmountPaid-inv.Refunded()))}
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
	if err != nil || !found {
		return nil, saleRefusal{saleUnavailable, "the payment could not be read; try again"}
	}
	if err := takeOrder(ctx, db, where, "refund.wire", "refund:"+ref, ref, inv.PaymentRef, currency.Cents(amount), inv.Currency, "", 0, time.Time{}); err != nil {
		return nil, saleRefusal{saleUnavailable, err.Error()}
	}
	if err := inv.GetById(inv.Id()); err != nil {
		return nil, saleRefusal{saleUnavailable, "the invoice could not be read; try again"}
	}
	return orderView(inv, o, nil, nil), nil
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
			view, err := VoidOrder(ctx, org, inv.Id(), "unpaid past due")
			if err != nil {
				log.Error("invoice expiry: invoice %s of %s was not voided: %v", inv.Id(), org.Name, err)
				continue
			}
			out = append(out, Expired{Org: org.Name, Order: view})
		}
	}
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

// Hosted is an order's invoice as anyone holding its link reads it: who bills
// whom for what, when it is due, and, while it is open, how to pay it.
type Hosted struct {
	*InvoiceOrder
	Issuer        string   `json:"issuer"`
	IssuerAddress string   `json:"issuerAddress,omitempty"`
	BillTo        string   `json:"billTo"`
	HowToPay      []string `json:"howToPay,omitempty"`
}

// HostedInvoice reads the order invoice a link shows. An unknown link is a miss,
// never a hint that one exists.
func HostedInvoice(ctx context.Context, link string) (*Hosted, error) {
	org, inv, o, err := linked(ctx, link)
	if err != nil {
		return nil, err
	}
	is := issuerFor(checkout.BrandSlugForHost(o.Host))
	h := &Hosted{
		InvoiceOrder: orderView(inv, o, nil, nil), Issuer: is.legal, IssuerAddress: is.address,
		BillTo: billedName(org, inv),
	}
	if inv.Status == billinginvoice.Open {
		pay, err := payOf(ctx, inv, o)
		if err != nil {
			return nil, err
		}
		h.Wire = &pay.wire
		h.HowToPay = pay.steps(fmtAmount(inv.Currency, inv.AmountDue), fmtDate(inv.DueDate))
	}
	return h, nil
}

// HostedInvoicePDF renders the order invoice a link shows, with how to pay it
// while it is open.
func HostedInvoicePDF(ctx context.Context, link string) (*Document, error) {
	org, inv, o, err := linked(ctx, link)
	if err != nil {
		return nil, err
	}
	var pay *wirePay
	if inv.Status == billinginvoice.Open {
		if pay, err = payOf(ctx, inv, o); err != nil {
			return nil, err
		}
	}
	return &Document{Filename: inv.NumberStr, Body: renderInvoicePDF(inv, billedName(org, inv), checkout.BrandSlugForHost(o.Host), pay)}, nil
}

// linked resolves a link to the order invoice it shows.
func linked(ctx context.Context, link string) (*organization.Organization, *billinginvoice.BillingInvoice, *order, error) {
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

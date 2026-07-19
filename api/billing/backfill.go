package billing

// One-time, idempotent backfill of the customer-activity spine. The live
// emitters (events_emit.go) only capture NEW billing activity; this replays the
// EXISTING subscriptions, invoices, and api-usage transactions of every tenant
// through the SAME event envelopes so the warehouse (commerce.events) shows real
// HISTORY, which admin.hanzo.ai's warehouse-backed billing views read.
//
// DRY: event SHAPING is not duplicated — the pure planners below reuse the
// events_emit.go builders (subscriptionEvent / invoiceEvent) and a faithful
// inverse of the usage write path (usageEventFromTxn), then hand the typed
// payload to the events client's Backfill* methods, which build the identical
// envelope the live path posts.
//
// IDEMPOTENT: every replayed event carries a deterministic id (BackfillEventID)
// derived only from immutable fields of the source record — org, entity id, event
// name, transition time — so a second run recomputes the SAME id per
// (entity, lifecycle-transition) and cannot double-count. See BackfillEventID and
// events.Client.emitBackfill for how the id reaches (and is deduped in) the
// warehouse.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/events"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/models/transaction"
)

// BackfillOptions bounds one backfill run.
type BackfillOptions struct {
	Org    string         // single org name to replay; empty = every org
	Since  time.Time      // only replay transitions at/after this instant; zero = all history
	DryRun bool           // count only, emit NOTHING
	Events *events.Client // collector client; nil (or DryRun) => nothing is posted
}

// OrgBackfill is one org's tally: how many source rows were scanned and, per
// event name, how many lifecycle events were (or in dry-run WOULD be) emitted.
type OrgBackfill struct {
	Org           string
	Subscriptions int            // subscription rows scanned
	Invoices      int            // invoice rows scanned
	Usage         int            // api-usage transaction rows scanned
	Emitted       map[string]int // event name -> count
	Total         int
}

// BackfillReport is the run summary across every org walked.
type BackfillReport struct {
	DryRun bool
	Orgs   []OrgBackfill
	Totals map[string]int // event name -> fleet total
	Total  int
}

// BackfillEventID derives the STABLE id for one (org, entity, lifecycle
// transition). It is a SHA-256 over the org name, the entity id, the event name,
// and the transition time in unix-millis — every field immutable on a historical
// record — so a re-run recomputes the identical id and cannot double-count.
// Unit-separated (0x1f) so no two distinct inputs collide by concatenation. This
// is set as the commerce.events event_id (its ORDER BY key) and mirrored into
// properties.event_id for read-side dedup.
func BackfillEventID(orgName, entityID, eventType string, at time.Time) string {
	h := sha256.New()
	for _, part := range []string{orgName, entityID, eventType, strconv.FormatInt(at.UTC().UnixMilli(), 10)} {
		_, _ = io.WriteString(h, part)
		_, _ = h.Write([]byte{0x1f})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// plannedEvent is one replayable lifecycle transition: its canonical name, the
// deterministic id, the historical instant it happened, and a best-effort emit.
type plannedEvent struct {
	Type string
	ID   string
	At   time.Time
	fire func(ctx context.Context, ev *events.Client) error
}

// planSubscription maps one subscription record onto its derivable lifecycle
// transitions:
//   - created:   always, at CreatedAt (else Start).
//   - renewed:   ONCE at the current period start when that start is at least ~one
//     billing interval after creation — proof the sub advanced past its first
//     period. (The full per-cycle history is reconstructed from invoice_paid, not
//     this single row, so we emit at most one marker here — see renewalTime.)
//   - canceled:  when the record is canceled, at CanceledAt (else Ended, else CreatedAt).
//
// plan_changed is NOT derivable — the row holds only the CURRENT plan, no prior
// state (models/billingevent is not written on plan change) — so it is
// deliberately never emitted. Bundle-child subs (the entitlement shadow of a paid
// parent, price 0) are skipped, exactly like the metrics fold.
func planSubscription(orgName string, s *subscription.Subscription) []plannedEvent {
	if s.ProviderType == "bundle" {
		return nil
	}
	payload := subscriptionEvent(orgName, s) // reuse the events_emit.go builder (DRY)
	var out []plannedEvent
	add := func(eventName string, at time.Time) {
		if at.IsZero() {
			return
		}
		id := BackfillEventID(orgName, s.Id(), eventName, at)
		out = append(out, plannedEvent{Type: eventName, ID: id, At: at,
			fire: func(ctx context.Context, ev *events.Client) error {
				return ev.BackfillSubscription(ctx, eventName, id, at, payload)
			}})
	}
	add(events.EventSubscriptionCreated, firstNonZero(s.CreatedAt, s.Start))
	if rt, ok := renewalTime(s); ok {
		add(events.EventSubscriptionRenewed, rt)
	}
	if s.Canceled || s.Status == subscription.Canceled {
		add(events.EventSubscriptionCanceled, firstNonZero(s.CanceledAt, s.Ended, s.CreatedAt))
	}
	return out
}

// planInvoice maps one invoice record onto its lifecycle transitions by status:
//   - finalized: any invoice past draft (open/paid/void/uncollectible), at
//     CreatedAt (the model has no separate finalized-at column, so creation is the
//     best datable finalize instant).
//   - paid:      a paid invoice, at PaidAt (else CreatedAt).
//   - void:      a void invoice, at VoidedAt (else CreatedAt).
func planInvoice(orgName string, inv *billinginvoice.BillingInvoice) []plannedEvent {
	payload := invoiceEvent(orgName, inv) // reuse the events_emit.go builder (DRY)
	var out []plannedEvent
	add := func(eventName string, at time.Time) {
		if at.IsZero() {
			return
		}
		id := BackfillEventID(orgName, inv.Id(), eventName, at)
		out = append(out, plannedEvent{Type: eventName, ID: id, At: at,
			fire: func(ctx context.Context, ev *events.Client) error {
				return ev.BackfillInvoice(ctx, eventName, id, at, payload)
			}})
	}
	if inv.Status != billinginvoice.Draft {
		add(events.EventInvoiceFinalized, inv.CreatedAt)
	}
	switch inv.Status {
	case billinginvoice.Paid:
		add(events.EventInvoicePaid, firstNonZero(inv.PaidAt, inv.CreatedAt))
	case billinginvoice.Void:
		add(events.EventInvoiceVoid, firstNonZero(inv.VoidedAt, inv.CreatedAt))
	}
	return out
}

// planUsage maps one api-usage transaction onto its single debit event, at the
// transaction's CreatedAt.
func planUsage(orgName string, t *transaction.Transaction) []plannedEvent {
	if t.Tags != "api-usage" || t.CreatedAt.IsZero() {
		return nil
	}
	payload := usageEventFromTxn(orgName, t)
	id := BackfillEventID(orgName, t.Id(), events.EventAPIUsageDebit, t.CreatedAt)
	return []plannedEvent{{Type: events.EventAPIUsageDebit, ID: id, At: t.CreatedAt,
		fire: func(ctx context.Context, ev *events.Client) error {
			return ev.BackfillAPIUsage(ctx, id, t.CreatedAt, payload)
		}}}
}

// usageEventFromTxn is the inverse of the usage write path (api/billing/usage.go):
// it reconstructs the metered-debit event from a stored api-usage transaction. The
// user is the debited wallet (SourceId; DestinationId on a legacy row); the
// model/provider/tokens/requestId/status ride in Metadata; amountMicros carries
// the exact sub-cent debit (falling back to cents*10_000). This adapts a stored
// SOURCE onto the existing events.APIUsage shape — it does not re-shape the event.
func usageEventFromTxn(orgName string, t *transaction.Transaction) *events.APIUsage {
	user := t.SourceId
	if user == "" {
		user = t.DestinationId
	}
	cents := int64(t.Amount)
	return &events.APIUsage{
		OrgID:        orgName,
		UserID:       user,
		AmountCents:  cents,
		AmountMicros: metaInt64(t.Metadata, "amountMicros", cents*10000),
		Model:        metaString(t.Metadata, "model"),
		Provider:     metaString(t.Metadata, "provider"),
		Project:      t.Project,
		Service:      t.Service,
		RequestID:    metaString(t.Metadata, "requestId"),
		TotalTokens:  int(metaInt64(t.Metadata, "totalTokens", 0)),
		Status:       metaString(t.Metadata, "status"),
	}
}

// renewalTime returns the current period start when the subscription has clearly
// rolled past its first billing period — PeriodStart is at least ~one interval
// after creation — and the sub is live (active or past_due). Only then is a
// renewal a fact derivable from this single row; otherwise (false) none is emitted
// (a first-period billing-anchor offset must not be counted as a renewal).
func renewalTime(s *subscription.Subscription) (time.Time, bool) {
	if s.Status != subscription.Active && s.Status != subscription.PastDue {
		return time.Time{}, false
	}
	created := firstNonZero(s.CreatedAt, s.Start)
	if created.IsZero() || s.PeriodStart.IsZero() {
		return time.Time{}, false
	}
	interval := intervalDuration(string(s.Plan.Interval))
	if s.PeriodStart.Sub(created) < time.Duration(float64(interval)*0.9) {
		return time.Time{}, false
	}
	return s.PeriodStart, true
}

// intervalDuration is the approximate length of one billing interval, used only to
// gate the renewed derivation (never for money math).
func intervalDuration(interval string) time.Duration {
	switch strings.ToLower(strings.TrimSpace(interval)) {
	case "year", "yearly", "annual", "annually":
		return 365 * 24 * time.Hour
	case "week", "weekly":
		return 7 * 24 * time.Hour
	case "day", "daily":
		return 24 * time.Hour
	default: // month/monthly and anything unrecognized
		return 30 * 24 * time.Hour
	}
}

// BackfillEvents walks every organization once — root-namespace org list, then
// each org's own namespace — the SAME walk api/metrics.buildRollup uses, and
// replays each org's existing subscriptions, invoices, and api-usage transactions
// as lifecycle events. In DryRun it only tallies what WOULD be emitted; otherwise
// it fires each event best-effort through the deterministic-id backfill emit.
func BackfillEvents(ctx context.Context, opts BackfillOptions) (*BackfillReport, error) {
	rootDb := datastore.New(ctx)
	orgs := make([]*organization.Organization, 0)
	if _, err := organization.Query(rootDb).GetAll(&orgs); err != nil {
		return nil, fmt.Errorf("list organizations: %w", err)
	}
	report := &BackfillReport{DryRun: opts.DryRun, Totals: map[string]int{}}
	for _, org := range orgs {
		if opts.Org != "" && org.Name != opts.Org {
			continue
		}
		ob := backfillOrg(ctx, opts, org)
		report.Orgs = append(report.Orgs, ob)
		for name, n := range ob.Emitted {
			report.Totals[name] += n
		}
		report.Total += ob.Total
	}
	return report, nil
}

// backfillOrg replays one org's namespace. Read failures on any entity kind are
// tolerated (an empty/unreadable tenant contributes nothing — an honest zero,
// never a fabricated figure), mirroring buildRollup.
func backfillOrg(ctx context.Context, opts BackfillOptions, org *organization.Organization) OrgBackfill {
	ob := OrgBackfill{Org: org.Name, Emitted: map[string]int{}}
	db := datastore.New(org.Namespaced(ctx))

	subs := make([]*subscription.Subscription, 0)
	if _, err := subscription.Query(db).GetAll(&subs); err == nil {
		ob.Subscriptions = len(subs)
		for _, s := range subs {
			ob.apply(ctx, opts, planSubscription(org.Name, s))
		}
	}

	invs := make([]*billinginvoice.BillingInvoice, 0)
	if _, err := billinginvoice.Query(db).GetAll(&invs); err == nil {
		ob.Invoices = len(invs)
		for _, inv := range invs {
			ob.apply(ctx, opts, planInvoice(org.Name, inv))
		}
	}

	txns := make([]*transaction.Transaction, 0)
	if _, err := transaction.Query(db).Filter("Tags=", "api-usage").GetAll(&txns); err == nil {
		ob.Usage = len(txns)
		for _, t := range txns {
			ob.apply(ctx, opts, planUsage(org.Name, t))
		}
	}
	return ob
}

// apply records (and, unless dry-run, fires) the planned events for one entity,
// honoring the Since window. Emission is best-effort: a collector error is
// swallowed so it never aborts the walk — mirroring the fire-and-forget live
// emitters. The per-call HTTP timeout (events.Client) bounds each fire so a down
// collector can never block the run indefinitely.
func (ob *OrgBackfill) apply(ctx context.Context, opts BackfillOptions, planned []plannedEvent) {
	for _, ev := range planned {
		if !opts.Since.IsZero() && ev.At.Before(opts.Since) {
			continue
		}
		ob.Emitted[ev.Type]++
		ob.Total++
		if opts.DryRun || opts.Events == nil {
			continue
		}
		_ = ev.fire(ctx, opts.Events)
	}
}

// ── small typed-map readers (Metadata is JSON-decoded, so numbers arrive as
// float64 or json.Number depending on the decoder — handle both) ──────────────

func metaString(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	if s, ok := m[key].(string); ok {
		return s
	}
	return ""
}

func metaInt64(m map[string]interface{}, key string, def int64) int64 {
	if m == nil {
		return def
	}
	switch v := m[key].(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case float64:
		return int64(v)
	case json.Number:
		if n, err := v.Int64(); err == nil {
			return n
		}
	case string:
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return def
}

// firstNonZero returns the first non-zero time (its arguments are candidate
// transition timestamps in preference order).
func firstNonZero(ts ...time.Time) time.Time {
	for _, t := range ts {
		if !t.IsZero() {
			return t
		}
	}
	return time.Time{}
}

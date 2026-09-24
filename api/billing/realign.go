package billing

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/billing/engine"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/idempotencykey"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/util/json/http"
)

// RealignResult is one subscription the realignment moved back onto the period
// its paid invoice covers, or in a dry run would move. Lapses says the move
// leaves it past that period and the grace after it (engine.Lapsed): it will
// confer nothing until its customer renews it or subscribes again.
type RealignResult struct {
	Org            string    `json:"org"`
	SubscriptionId string    `json:"subscriptionId"`
	UserId         string    `json:"userId"`
	Plan           string    `json:"plan"`
	Interval       string    `json:"interval"`
	InvoiceId      string    `json:"invoiceId"`
	FromStart      time.Time `json:"fromStart"`
	FromEnd        time.Time `json:"fromEnd"`
	ToStart        time.Time `json:"toStart"`
	ToEnd          time.Time `json:"toEnd"`
	Lapses         bool      `json:"lapses"`
}

// RealignReport is one realignment run: every subscription it moved, or in a
// dry run would move, out of the live ones it considered.
type RealignReport struct {
	DryRun     bool            `json:"dryRun"`
	Orgs       int             `json:"orgs"`
	Considered int             `json:"considered"`
	Realigned  int             `json:"realigned"`
	Lapsing    int             `json:"lapsing"`
	Results    []RealignResult `json:"results"`
	Errors     []string        `json:"errors,omitempty"`
}

// Realign is the one-time migration that corrects the dates of the
// subscriptions in orgs that sit a period ahead of the invoice that paid for
// them (engine.Realign): each moves back onto that invoice's period, is renewed
// when that period ends, and is marked so it is never moved again. It is an
// operator's act, run on its own and never as part of reading or renewing a
// subscription. With dryRun every move is reported and nothing is written.
//
// A real run over an org, or the first live cycle there that finds nothing to
// realign, records when it ran (realignCutover), and from then on no invoice
// issued after it makes a row a candidate. A row that is never
// charged (comped) is never a candidate. Each org's run holds the org's cycle
// lock, so it never overlaps a cycle run of the same org in this process.
func Realign(ctx context.Context, orgs []*organization.Organization, dryRun bool) *RealignReport {
	report := &RealignReport{DryRun: dryRun, Orgs: len(orgs), Results: make([]RealignResult, 0)}
	for _, org := range orgs {
		realignOrg(ctx, org, dryRun, report)
	}
	return report
}

func realignOrg(ctx context.Context, org *organization.Organization, dryRun bool, report *RealignReport) {
	defer lockOrgCycle(org.Name)()
	fail := func(what string, err error) {
		log.Error("realign: %s in %q: %v", what, org.Name, err)
		report.Errors = append(report.Errors, fmt.Sprintf("%s: %s: %v", org.Name, what, err))
	}
	db := datastore.New(org.Namespaced(ctx))
	cutover, err := realignCutover(db)
	if err != nil {
		fail("read the realignment's cutover", err)
		return
	}
	subs, err := orgSubscriptions(db, "")
	if err != nil {
		fail("list subscriptions", err)
		return
	}
	now, failed := time.Now(), false
	for _, s := range subs {
		if isBundleRow(s) || !engine.Live(s) {
			continue
		}
		report.Considered++
		if comped, err := compedRow(org, db, s); err != nil || comped {
			if err != nil {
				fail("subscription "+s.Id(), err)
				failed = true
			}
			continue
		}
		r, err := engine.Realign(db, s, cutover, dryRun)
		if err != nil {
			fail("subscription "+s.Id(), err)
			failed = true
			continue
		}
		if r == nil {
			continue
		}
		moved := *s
		moved.PeriodStart, moved.PeriodEnd = r.ToStart, r.ToEnd
		lapses := engine.Lapsed(&moved, now)
		report.Realigned++
		if lapses {
			report.Lapsing++
		}
		report.Results = append(report.Results, RealignResult{
			Org: org.Name, SubscriptionId: s.Id(), UserId: s.UserId, Plan: planOf(s), Interval: string(s.Plan.Interval),
			InvoiceId: r.InvoiceId, FromStart: r.FromStart, FromEnd: r.FromEnd, ToStart: r.ToStart, ToEnd: r.ToEnd, Lapses: lapses,
		})
	}
	if !dryRun && !failed && cutover.IsZero() {
		if err := markRealigned(db, now); err != nil {
			fail("record the realignment's cutover", err)
		}
	}
}

// compedRow reports whether s is never charged (renewalOf answers comped): an
// ecosystem org's own plan provisioned on enterprise terms. Its period is moved
// on by the cycle, never by a charge, so it is never a realignment candidate.
func compedRow(org *organization.Organization, db *datastore.Datastore, s *subscription.Subscription) (bool, error) {
	_, source, err := renewalOf(org, db, s, rail{}, nil)
	return source == sourceComped, err
}

// awaitingRealign reports whether s is a realignment candidate in org: not a
// seat, live, not comped, and matching engine.RealignPending under the org's
// cutover.
func awaitingRealign(org *organization.Organization, db *datastore.Datastore, s *subscription.Subscription, cutover time.Time) (bool, error) {
	if isBundleRow(s) || !engine.Live(s) {
		return false, nil
	}
	if comped, err := compedRow(org, db, s); err != nil || comped {
		return false, err
	}
	return engine.RealignPending(db, s, cutover)
}

// The org's realignment record: when its cutover was recorded.
const (
	realignScope = "billing-realign"
	realignKey   = "cutover"
)

// realignCutover is when the org's realignment first ran for real, or a live
// cycle first found nothing there to realign; zero before either.
func realignCutover(db *datastore.Datastore) (time.Time, error) {
	rec := idempotencykey.New(db)
	err := rec.Get(db.NewKey(rec.Kind(), idempotencykey.DeterministicID(realignScope, realignKey), 0, nil))
	switch {
	case errors.Is(err, datastore.ErrNoSuchEntity):
		return time.Time{}, nil
	case err != nil:
		return time.Time{}, err
	case rec.Status != idempotencykey.StatusCompleted:
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339Nano, rec.Response)
}

// markRealigned records the org's cutover at now, once.
func markRealigned(db *datastore.Datastore, now time.Time) error {
	rec, replay, err := idempotencykey.Begin(db, realignScope, realignKey)
	if err != nil || (replay && rec.Status == idempotencykey.StatusCompleted) {
		return err
	}
	return idempotencykey.Complete(rec, now.UTC().Format(time.RFC3339Nano))
}

// RealignSubscriptions runs the realignment for the request's organization: a
// dry run unless the request says dryRun=false.
//
//	POST /v1/billing/realign/run[?dryRun=false]
func RealignSubscriptions(c *zip.Ctx) error {
	dryRun, err := dryRunOf(c)
	if err != nil {
		return http.Fail(c, 400, err.Error(), nil)
	}
	org := middleware.GetOrganization(c)
	return c.JSON(200, Realign(context.WithoutCancel(c.Context()), []*organization.Organization{org}, dryRun))
}

// RealignSubscriptionsAllOrgs runs the realignment for every organization: a dry
// run unless the request says dryRun=false.
//
//	POST /v1/billing/realign/run-all[?dryRun=false]
func RealignSubscriptionsAllOrgs(c *zip.Ctx) error {
	dryRun, err := dryRunOf(c)
	if err != nil {
		return http.Fail(c, 400, err.Error(), nil)
	}
	ctx := context.WithoutCancel(c.Context())
	orgs := make([]*organization.Organization, 0)
	if _, err := organization.Query(datastore.New(ctx)).GetAll(&orgs); err != nil {
		log.Error("Failed to list organizations for realignment: %v", err, c)
		return http.Fail(c, 500, "failed to list organizations", err)
	}
	return c.JSON(200, Realign(ctx, orgs, dryRun))
}

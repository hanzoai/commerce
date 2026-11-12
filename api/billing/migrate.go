package billing

// migrate.go — moving every holder of a retired personal rung onto the ladder the
// catalog sells (owner, 2026-11-12: "just migrate them all to the new plans").
//
// A holder of a personal plan that is no longer on sale — a rung the catalog
// archived (dev, max-5x, max-20x, max-50x), a slug it renamed (pro-2x, max-10x),
// one it retired long ago (max), or the $9 Go the $8 Go reuses the slug of — is
// moved onto a rung of the ladder:
//
//   - where the catalog names the rung that replaces theirs (`replaces`), onto it,
//     when it is on sale and lists at or below what they pay;
//   - otherwise onto the dearest rung on sale whose monthly list price is at or
//     below what they pay a month now.
//
// A rung is a plan the catalog states a usage multiple for (staticPlan.Multiple),
// so the ladder runs Go to Ultimate; Unlimited states no caps and no multiple, and
// nobody is moved onto it. Nobody pays more: the rung moved onto lists at or below
// what the holder pays, and a price staff agreed below the new list price is kept.
//
// The plan and what it entitles switch when the move runs; the price moved onto
// is charged from the next renewal, because the period in hand was paid. A yearly
// holder keeps the year paid for and then renews monthly on the new rung. A holder
// paying less than the cheapest rung, or nothing, is left as they are.
//
// It runs on every boot, with the seed (SeedPlans), and is idempotent: a holder
// moved is on a rung on sale and is not moved again. Each move is written under
// the subject's sale lease and recorded as a subscription.migrated billing event,
// the audit record, with the plan and the price before and after. It switches no
// reload on and sends nobody anything.

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/hanzoai/commerce/billing/engine"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/customerprice"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/plan"
	"github.com/hanzoai/commerce/models/subscription"
	types "github.com/hanzoai/commerce/types"
)

// Migrated is one holder moved onto the ladder.
type Migrated struct {
	Org            string `json:"org"`
	Subject        string `json:"subject"`
	SubscriptionID string `json:"subscriptionId"`
	// From and To are the plan slugs before and after.
	From string `json:"from"`
	To   string `json:"to"`
	// FromCents is what the holder paid for one period of From, and FromInterval
	// that period; ToCents is what each monthly period of To charges from the next
	// renewal: its list price, or the price staff agreed when that is lower.
	FromCents    int64  `json:"fromCents"`
	FromInterval string `json:"fromInterval"`
	ToCents      int64  `json:"toCents"`
}

// Migration is what one run of the migration did.
type Migration struct {
	Moved []Migrated `json:"moved"`
	// Left counts the holders of a retired rung no rung fits: they pay less than
	// the cheapest rung on sale, or nothing.
	Left int `json:"left"`
	// Busy counts the holders whose sale lease was held, by a sale or another
	// replica's run; the next boot moves them.
	Busy   int      `json:"busy"`
	Errors []string `json:"errors,omitempty"`
}

// rung is a plan of the ladder a holder may be moved onto, as the plan authority
// sells it.
type rung struct {
	slug string
	row  *plan.Plan
}

// ladderOnSale is every rung of the ladder on sale, dearest first: a personal
// catalog plan that states a usage multiple, whose authority row is listed and
// charges.
func ladderOnSale(ctx context.Context) []rung {
	out := make([]rung, 0)
	for i := range catalog {
		sp := &catalog[i]
		if sp.Category != categoryPersonal || sp.Multiple <= 0 || !plan.ListedStatus(sp.Status) {
			continue
		}
		row := rowOf(ctx, sp.Slug)
		if row == nil || !row.Listed() || row.Price <= 0 {
			continue
		}
		out = append(out, rung{slug: sp.Slug, row: row})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].row.Price > out[j].row.Price })
	return out
}

// monthlyCents is what s pays a month for its plan: its period's price — the
// listed price for a comp, which pays nothing — over the months the period runs.
func monthlyCents(s *subscription.Subscription) int64 {
	cents := int64(s.Plan.Price)
	if comped(s) {
		cents = metaInt64(s.Metadata, "listCents", 0)
	}
	months := int64(max(s.Plan.IntervalCount, 1))
	if s.Plan.Interval == types.Yearly || string(s.Plan.Interval) == "year" {
		months *= 12
	}
	return cents / months
}

// comped reports whether s is a comp: recorded, paid by nobody.
func comped(s *subscription.Subscription) bool {
	return s.Type == subscription.External && s.ProviderType == processorComp
}

// target is the rung a holder of slug paying pay a month is moved onto, or nil
// when none fits. A slug the catalog names a replacement for follows it while
// that rung is on sale and lists at or below pay; otherwise the dearest rung on
// sale listed at or below pay.
func target(slug string, pay int64, ladder []rung) *rung {
	for seen := map[string]bool{}; !seen[slug]; {
		seen[slug] = true
		next, ok := successor[slug]
		if !ok {
			break
		}
		slug = next
		for i := range ladder {
			if ladder[i].slug == slug && int64(ladder[i].row.Price) <= pay {
				return &ladder[i]
			}
		}
	}
	for i := range ladder {
		if int64(ladder[i].row.Price) <= pay {
			return &ladder[i]
		}
	}
	return nil
}

// retired reports whether s holds a personal rung the ladder no longer sells as s
// holds it:
//
//   - a rung on sale held at a price above the one it lists at (the $9 Go, whose
//     slug the $8 Go reuses);
//   - a slug the catalog names a replacement for (pro-2x, max-10x, max);
//   - a plan whose authority row is archived, or that has no row and that the
//     catalog still resolves.
//
// A plan on sale that is no rung, a private plan made for one customer, or a
// draft is not retired: it is not the catalog's to move.
func retired(ctx context.Context, s *subscription.Subscription, ladder []rung) bool {
	slug := subscriptionSlug(s)
	for _, r := range ladder {
		if r.slug == slug {
			return !comped(s) && len(r.row.Prices) == 0 && s.Plan.Interval == r.row.Interval && s.Plan.Price > r.row.Price
		}
	}
	category := s.Plan.Category
	_, renamed := successor[slug]
	row := rowOf(ctx, slug)
	switch {
	case renamed:
	case row != nil:
		if row.Status != plan.StatusArchived {
			return false
		}
	case lookupPlan(slug) == nil:
		return false
	}
	if category == "" && row != nil {
		category = row.Category
	}
	if category == "" {
		if sp := lookupPlan(slug); sp != nil {
			category = sp.Category
		}
	}
	return category == categoryPersonal
}

// movable reports whether s is a live holding the migration moves: one that
// renews or is paid for (active, past due, or waiting to start on a card), not
// over, not a bundle row, and of one seat type.
func movable(s *subscription.Subscription, now time.Time) bool {
	switch s.Status {
	case subscription.Active, subscription.PastDue, subscription.Scheduled:
	default:
		return false
	}
	return !engine.Over(s, now) && s.ProviderType != "bundle" && len(s.Lines) == 0
}

// MigrateRetired moves every holder of a retired personal rung, in every org,
// onto the ladder the catalog sells (see the file comment). It is safe to run on
// every boot: what it moved it never moves again.
func MigrateRetired(ctx context.Context) (*Migration, error) {
	orgs := make([]*organization.Organization, 0)
	if _, err := organization.Query(datastore.New(ctx)).GetAll(&orgs); err != nil {
		return nil, fmt.Errorf("migrate: list organizations: %w", err)
	}
	ladder := ladderOnSale(ctx)
	out := &Migration{Moved: []Migrated{}}
	if len(ladder) == 0 {
		return out, nil
	}
	now := time.Now()
	for _, org := range orgs {
		subs, err := ListSubscriptions(ctx, org, "", "")
		if err != nil {
			out.Errors = append(out.Errors, fmt.Sprintf("%s: subscriptions could not be read: %v", org.Name, err))
			continue
		}
		for _, s := range subs {
			if !movable(s, now) || !retired(ctx, s, ladder) {
				continue
			}
			to := target(subscriptionSlug(s), monthlyCents(s), ladder)
			if to == nil {
				out.Left++
				continue
			}
			m, err := migrateOne(ctx, org, s.Id(), to, ladder, now)
			switch {
			case errors.Is(err, errBusy):
				out.Busy++
			case err != nil:
				out.Errors = append(out.Errors, fmt.Sprintf("%s: subscription %s: %v", org.Name, s.Id(), err))
			case m != nil:
				out.Moved = append(out.Moved, *m)
			}
		}
	}
	return out, nil
}

// errBusy is a holder whose sale lease another act holds.
var errBusy = errors.New("migrate: the subject's sale is in progress")

// migrateOne moves subscription id onto to under its subject's sale lease, read
// again under it: a row a sale or another run moved meanwhile is left as it now
// stands.
func migrateOne(ctx context.Context, org *organization.Organization, id string, to *rung, ladder []rung, now time.Time) (*Migrated, error) {
	db := datastore.New(org.Namespaced(ctx))
	s := subscription.New(db)
	if err := s.GetById(id); err != nil {
		return nil, err
	}
	lease, err := holdSale(db, s.UserId)
	if err != nil {
		if IsSaleConflict(err) {
			return nil, errBusy
		}
		return nil, err
	}
	defer lease.Release()
	if err := s.GetById(id); err != nil {
		return nil, err
	}
	if !movable(s, now) || !retired(ctx, s, ladder) {
		return nil, nil
	}

	from, fromInterval := subscriptionSlug(s), intervalOf(s.Plan.Interval)
	fromCents := int64(s.Plan.Price)
	if c := s.CustomerCents; c > 0 && c < fromCents {
		fromCents = c
	}
	list := int64(to.row.Price)
	agreed := int64(0)
	if c := s.CustomerCents; c > 0 && c < list {
		agreed = c
	}

	next := *to.row
	if comped(s) {
		// A comp moves as a comp: held at 0, with the price the rung lists at kept
		// beside it, which is what makes it a paid plan given away.
		next.Price = 0
		s.Metadata = mapOrNew(s.Metadata)
		s.Metadata["listCents"] = list
		fromCents, list, agreed = 0, 0, 0
	}
	if _, err := engine.ChangePlan(s, &next, false); err != nil {
		return nil, err
	}
	s.CustomerCents = agreed
	s.Metadata = mapOrNew(s.Metadata)
	s.Metadata["migratedFrom"] = from
	s.Metadata["migratedAt"] = now.UTC().Format(time.RFC3339)
	if err := s.Update(); err != nil {
		return nil, err
	}
	if agreed > 0 {
		if err := keepAgreed(db, s.UserId, to.slug, agreed, from); err != nil {
			log.Error("migrate: subscription %s carries its agreed %d cents on %s and the price was not recorded: %v", s.Id(), agreed, to.slug, err)
		}
	}
	follow(ctx, org, s.UserId, from, to.slug)

	toCents := list
	if agreed > 0 {
		toCents = agreed
	}
	m := &Migrated{
		Org: org.Name, Subject: s.UserId, SubscriptionID: s.Id(),
		From: from, To: to.slug, FromCents: fromCents, FromInterval: fromInterval, ToCents: toCents,
	}
	if _, err := engine.EmitBillingEvent(db, "subscription.migrated", "subscription", s.Id(), s.UserId,
		types.Map{"from": from, "to": to.slug, "fromCents": fromCents, "fromInterval": fromInterval, "toCents": toCents,
			"catalog": PinnedPlansVersion, "by": "migrateRetired"},
		types.Map{"plan": from}); err != nil {
		log.Error("migrate: subscription %s moved %s -> %s and its audit record was not written: %v", s.Id(), from, to.slug, err)
	}
	log.Info("migrate: %s/%s subscription %s moved %s (%d cents a %s) -> %s (%d cents a month from the next renewal)",
		org.Name, s.UserId, s.Id(), from, fromCents, fromInterval, to.slug, toCents)
	return m, nil
}

// keepAgreed records, for the rung a holder was moved onto, the price staff
// agreed with them, unless one is already agreed for it.
func keepAgreed(db *datastore.Datastore, subject, slug string, cents int64, from string) error {
	interval := intervalOf(types.Monthly)
	r := customerprice.New(db)
	err := r.Get(db.NewKey(r.Kind(), customerprice.ID(subject, slug, interval), 0, nil))
	switch {
	case err == nil:
		return nil
	case !errors.Is(err, datastore.ErrNoSuchEntity):
		return err
	}
	r.Subject, r.Plan, r.Interval, r.Cents = subject, slug, interval, cents
	r.Note = "agreed for " + from + ", kept when the subscription moved to " + slug
	r.SetId(customerprice.ID(subject, slug, interval))
	return r.Create()
}

// mapOrNew is m, or an empty map when m is nil.
func mapOrNew(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return map[string]interface{}{}
	}
	return m
}

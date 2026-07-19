// Command backfill-events replays commerce's EXISTING subscriptions, invoices,
// and api-usage transactions into the analytics collector (commerce.events) as
// lifecycle events — so admin.hanzo.ai's warehouse-backed billing views show real
// HISTORY, not just events emitted from now on.
//
// It is IDEMPOTENT: every event carries a deterministic id derived from the
// immutable source record (see billing.BackfillEventID), so a re-run recomputes
// the SAME id per (entity, lifecycle-transition) and never double-counts.
//
// Usage:
//
//	backfill-events                                  # DRY-RUN every org (default), emits nothing
//	backfill-events --org acme                       # DRY-RUN one org
//	backfill-events --since 2026-01-01T00:00:00Z     # bound the window
//	# real emit (per-org first), after the collector is deployed:
//	COMMERCE_BACKFILL_ALLOW=true backfill-events --org acme \
//	    --dry-run=false --endpoint http://analytics-collector.hanzo.svc:8091
//
// Like cmd/grant, it bootstraps the same datastore commerce uses at runtime
// (commerce.App.Bootstrap) then calls billing.BackfillEvents directly. --dry-run
// defaults ON; a live emit additionally requires --endpoint (or ANALYTICS_ENDPOINT)
// and COMMERCE_BACKFILL_ALLOW=true so it can never fire against the wrong env by
// accident.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	commerce "github.com/hanzoai/commerce"
	"github.com/hanzoai/commerce/api/billing"
	"github.com/hanzoai/commerce/events"
)

func main() {
	var (
		org      = flag.String("org", "", "single org name to backfill (empty = every org)")
		dryRun   = flag.Bool("dry-run", true, "count only; emit NOTHING (default true). Pass --dry-run=false for a real emit.")
		sinceStr = flag.String("since", "", "RFC3339 lower bound on transition time (empty = all history)")
		endpoint = flag.String("endpoint", os.Getenv("ANALYTICS_ENDPOINT"), "analytics-collector base URL (required for a live emit)")
	)
	flag.Parse()

	var since time.Time
	if s := strings.TrimSpace(*sinceStr); s != "" {
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			die("invalid --since %q: %v (want RFC3339, e.g. 2026-01-01T00:00:00Z)", s, err)
		}
		since = t
	}

	// Bootstrap the same datastore commerce uses at runtime (DB, resolvers).
	app := commerce.New()
	if err := app.Bootstrap(); err != nil {
		die("commerce bootstrap failed: %v", err)
	}

	// Wire the collector only for a live emit — and refuse unless explicitly
	// confirmed, so this can never fire against a prod collector by accident.
	var client *events.Client
	if !*dryRun {
		ep := strings.TrimSpace(*endpoint)
		if ep == "" {
			die("a live emit requires --endpoint or ANALYTICS_ENDPOINT (the analytics-collector base URL)")
		}
		if os.Getenv("COMMERCE_BACKFILL_ALLOW") != "true" {
			die("refusing a live emit: set COMMERCE_BACKFILL_ALLOW=true to confirm the target env (or drop --dry-run=false)")
		}
		client = events.NewClient(ep)
	}

	fmt.Printf("backfill-events: %s | %s | %s\n\n", modeLabel(*dryRun, *endpoint), scopeLabel(*org), windowLabel(since))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	report, err := billing.BackfillEvents(ctx, billing.BackfillOptions{
		Org:    strings.TrimSpace(*org),
		Since:  since,
		DryRun: *dryRun,
		Events: client,
	})
	if err != nil {
		die("backfill failed: %v", err)
	}

	printReport(report)

	if client != nil {
		_ = client.Flush()
		_ = client.Close()
	}
}

func printReport(r *billing.BackfillReport) {
	sort.Slice(r.Orgs, func(i, j int) bool { return r.Orgs[i].Total > r.Orgs[j].Total })
	printed := 0
	for _, o := range r.Orgs {
		if o.Total == 0 {
			continue // don't spam empty tenants
		}
		fmt.Printf("  %-28s subs=%-4d inv=%-4d usage=%-5d -> %d events  %s\n",
			o.Org, o.Subscriptions, o.Invoices, o.Usage, o.Total, counts(o.Emitted))
		printed++
	}
	if printed == 0 {
		fmt.Println("  (no org has any replayable billing history in scope)")
	}
	fmt.Printf("\nTOTAL %d events across %d org(s):\n", r.Total, len(r.Orgs))
	for _, name := range sortedKeys(r.Totals) {
		fmt.Printf("  %-28s %d\n", name, r.Totals[name])
	}
	if r.DryRun {
		fmt.Printf("\n[dry-run] nothing was emitted. Re-run with --dry-run=false (+ --endpoint, COMMERCE_BACKFILL_ALLOW=true) to emit.\n")
	}
}

func counts(m map[string]int) string {
	parts := make([]string, 0, len(m))
	for _, k := range sortedKeys(m) {
		parts = append(parts, fmt.Sprintf("%s=%d", strings.TrimPrefix(k, "subscription_"), m[k]))
	}
	return "[" + strings.Join(parts, " ") + "]"
}

func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func modeLabel(dryRun bool, endpoint string) string {
	if dryRun {
		return "DRY-RUN (emitting nothing)"
	}
	return "LIVE -> " + strings.TrimSpace(endpoint)
}

func scopeLabel(org string) string {
	if o := strings.TrimSpace(org); o != "" {
		return "org=" + o
	}
	return "all orgs"
}

func windowLabel(since time.Time) string {
	if since.IsZero() {
		return "all history"
	}
	return "since " + since.UTC().Format(time.RFC3339)
}

func die(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "backfill-events: "+format+"\n", args...)
	os.Exit(1)
}

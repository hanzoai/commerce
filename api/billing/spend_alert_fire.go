package billing

// Spend-alert FIRING — the "alert" half of a spend-alert (issue #70). A row is
// BOTH a hard cap (enforced PRE-request, read-only, on the hot metering-gate path
// by AuthorizeSpendCap) AND an alert that must FIRE when the scope's cumulative
// period spend crosses its soft-warn threshold or its cap.
//
// Firing belongs on the WRITE path, not the gate: the alert is a pure function of
// the ledger that just changed, so RecordUsage evaluates it exactly once — right
// after the debit — off the request's critical path (fire-and-forget) and
// DEBOUNCED so it fires at most once per (period, level) and RE-ARMS when the
// monthly window rolls over. The debounce reuses the SAME UTC-month Schedule the
// cap resets on (currentPeriod()), so a fired alert and a reset can never drift.
//
// Decomplected onto the primitives: Meter = the debited period spend
// (scopeSpentCents), Policy = the crossed level (levelFor, the exact boundaries
// the verdict + Budgets view derive from), Schedule = the UTC-month window
// (currentPeriod), Bus = the events collector (EmitRaw). No new counter, no new
// threshold math — every value is reused so an alert can never disagree with the
// `over`/`warn`/`spend_cap` a customer already sees.

import (
	"context"
	"strings"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/events"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/spendalert"
)

// alertLevel ranks how far a scope's period spend has crossed: none < warn < over.
// It is the ONE ladder both the fire decision and the debounce compare on, so a
// warn escalates to over exactly once and never regresses within a period.
type alertLevel int

const (
	levelNone alertLevel = iota
	levelWarn
	levelOver
)

func (l alertLevel) String() string {
	switch l {
	case levelOver:
		return "over"
	case levelWarn:
		return "warn"
	default:
		return "none"
	}
}

func parseLevel(s string) alertLevel {
	switch s {
	case "over":
		return levelOver
	case "warn":
		return levelWarn
	default:
		return levelNone
	}
}

// levelFor maps a scope's period spend against a row's thresholds to the crossed
// level, reusing the EXACT boundaries the cap verdict and the console view derive
// from — scopeExhausted for `over`, EffectiveSoftPct for `warn` — so a fired alert
// can never disagree with the flags the Budgets page shows. A row with no cap
// (Threshold<=0) is unlimited: nothing to cross.
func levelFor(a *spendalert.SpendAlert, spent int64) alertLevel {
	if a.Threshold <= 0 {
		return levelNone
	}
	if scopeExhausted(spent, a.Threshold) {
		return levelOver
	}
	if spent >= a.Threshold*int64(a.EffectiveSoftPct())/100 {
		return levelWarn
	}
	return levelNone
}

// triggerStamp encodes the fired (period, level) into TriggeredAt so the debounce
// is stateless AND self-resetting: e.g. "2026-07:over". A stamp from a prior period
// is stale, so the monthly window reset re-arms the alert with no extra state.
func triggerStamp(period string, l alertLevel) string { return period + ":" + l.String() }

// parseTriggerStamp splits a "period:level" stamp. A legacy bare timestamp (the
// field's original ISO form) has no ':level' suffix → level none, so it never
// suppresses a real crossing; the first crossing after this ships re-stamps it.
func parseTriggerStamp(s string) (period string, level alertLevel) {
	i := strings.LastIndexByte(s, ':')
	if i < 0 {
		return "", levelNone
	}
	return s[:i], parseLevel(s[i+1:])
}

// FireSpendAlerts is the exported trigger the co-resident HOST calls right after it
// records a usage debit on the finance path. Commerce's own RecordUsage — where the
// internal fire runs — is NOT on the unified binary's usage path (usage goes to the
// finance ledger), so the host invokes this to fire the alert on the SAME crossing,
// reading the host-injected period spend (SetPeriodSpendReader). Standalone commerce
// keeps firing from RecordUsage directly; this is a no-op-safe additional entry
// point (idempotent per (period,level) debounce), never blocking the money path.
func FireSpendAlerts(ctx context.Context, db *datastore.Datastore, orgName string, test bool, project, service string, ev *events.Client) {
	checkAndFireSpendAlerts(ctx, db, orgName, test, project, service, ev)
}

// checkAndFireSpendAlerts is the fire-and-forget alert pass RecordUsage runs after
// a debit. For every row COVERING the debit's (project, service) scope it computes
// the scope's fresh period spend (the same append-only ledger the gate reads) and,
// when the crossed level is NEW for the current period — a first crossing, or an
// escalation warn→over — it stamps TriggeredAt and emits spend_alert.triggered on
// the events bus. It never blocks the money path and only WRITES on an actual
// (rare, debounced) crossing. ev may be nil (no collector wired): the TriggeredAt
// stamp still lands so the Budgets UI reflects the fire; only the bus emit is
// skipped.
func checkAndFireSpendAlerts(ctx context.Context, db *datastore.Datastore, orgName string, test bool, project, service string, ev *events.Client) {
	rows, err := loadOrgScopes(db)
	if err != nil {
		log.Error("spend-alert fire: load scopes failed: %v", err)
		return
	}
	period := currentPeriod()
	spentBy := map[string]int64{} // memoize per (project,service) scope — one query each.
	for _, a := range rows {
		if a.Threshold <= 0 || !a.Covers(project, service) {
			continue
		}
		key := a.Project + "\x00" + a.Service
		spent, ok := spentBy[key]
		if !ok {
			v, serr := scopeSpentCents(db, test, a.Project, a.Service)
			if serr != nil {
				continue // cannot measure this scope now — do not fire on a guess.
			}
			spent, spentBy[key] = v, v
		}

		level := levelFor(a, spent)
		if level == levelNone {
			continue
		}
		if prevPeriod, prevLevel := parseTriggerStamp(a.TriggeredAt); prevPeriod == period && prevLevel >= level {
			continue // already fired this level (or higher) this period — debounced.
		}

		// Re-fetch by id for a live model handle: a GetAll-loaded row carries its
		// fields + Id() but not the orm key Update() needs (loadOrgScopes only ever
		// read rows before). Re-check the debounce on the FRESH row so a concurrent
		// debit that already fired this level makes this a no-op (idempotent race).
		row := spendalert.New(db)
		if gerr := row.GetById(a.Id()); gerr != nil {
			log.Error("spend-alert fire: reload %s failed: %v", a.Id(), gerr)
			continue
		}
		if prevPeriod, prevLevel := parseTriggerStamp(row.TriggeredAt); prevPeriod == period && prevLevel >= level {
			continue
		}
		row.TriggeredAt = triggerStamp(period, level)
		if uerr := saveAlert(db, row); uerr != nil {
			log.Error("spend-alert fire: stamp update failed: %v", uerr)
			continue
		}
		emitSpendAlertTriggered(ctx, ev, orgName, test, row, level, spent, period)
	}
}

// emitSpendAlertTriggered publishes ONE spend_alert.triggered event to the
// analytics collector (commerce.events) — the SAME bus the usage-debit + billing
// events ride, so admin.hanzo.ai's fleet read side sees every fired alert with no
// per-org fan-out. Money is USD cents end to end. No-op when no collector is wired.
func emitSpendAlertTriggered(ctx context.Context, ev *events.Client, orgName string, test bool, a *spendalert.SpendAlert, level alertLevel, spent int64, period string) {
	if ev == nil {
		return
	}
	_ = ev.EmitRaw(ctx, map[string]interface{}{
		"event":           "spend_alert_triggered",
		"distinct_id":     a.UserId,
		"organization_id": orgName,
		"properties": map[string]interface{}{
			"alert_id":        a.Id(),
			"level":           level.String(), // "warn" | "over"
			"project":         a.Project,
			"service":         a.Service,
			"threshold_cents": a.Threshold,
			"spent_cents":     spent,
			"enforce":         a.Enforce, // a hard cap that will 402 vs an alert-only row.
			"period":          period,
			"test":            test,
		},
	})
}

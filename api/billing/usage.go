package billing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/billing/engine"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/db"
	"github.com/hanzoai/commerce/events"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/idempotencykey"
	"github.com/hanzoai/commerce/models/meter"
	"github.com/hanzoai/commerce/models/spendalert"
	"github.com/hanzoai/commerce/models/transaction"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/json/http"

	. "github.com/hanzoai/commerce/types"
)

type usageRequest struct {
	User     string `json:"user"`
	Currency string `json:"currency"`
	Amount   int64  `json:"amount"` // cents (back-compat; prefer amountMicros)
	// AmountMicros is the debit in micro-USD (1e6 = $1). Preferred over Amount:
	// it carries sub-cent precision so tiny spends aren't lost to cent rounding.
	// Chat's tokenValue is already micro-USD (1e6 tokenCredits = $1), so it maps
	// 1:1. Zero/absent => fall back to Amount*10000.
	AmountMicros int64 `json:"amountMicros"`
	// CostMicros is what serving this call cost US (the provider's price), in
	// micro-USD. Recorded beside the charge so margin is a fact on the row.
	CostMicros int64 `json:"costMicros"`
	// PaidBy names what pays for the call: plan, prepaid, credits, line, or
	// hanzo (our own spend, still recorded and still charged to its account).
	PaidBy   string `json:"paidBy"`
	Model    string `json:"model"`
	Provider string `json:"provider"`
	// Project / Service attribute this spend to a scope for per-scope caps
	// (issue #70). Empty = the org-wide default scope.
	Project          string `json:"project"`
	Service          string `json:"service"`
	PromptTokens     int    `json:"promptTokens"`
	CompletionTokens int    `json:"completionTokens"`
	TotalTokens      int    `json:"totalTokens"`
	RequestID        string `json:"requestId"`
	Premium          bool   `json:"premium"`
	Stream           bool   `json:"stream"`
	Status           string `json:"status"`
	ClientIP         string `json:"clientIp"`
}

// meterUsage records this act on the "api-usage" meter, for an org whose plan
// prices usage from a meter rather than from the debit RecordUsage writes. No
// such meter → nothing to record.
//
// A METER EVENT'S VALUE IS A QUANTITY, AND A CHARGE IS NOT ONE. AggregateUsage
// sums Value and hands the sum to PricingRule.CalculateCost, so a value that is
// already money comes back out multiplied by a unit price — the invoice line
// charges for the charge. This wrote req.Amount, the debit in cents, which was
// wrong twice from one field: non-zero it double-charged, and on the preferred
// amountMicros path `amount` is absent, so the value was 0 and the line vanished.
// The quantity a token charge prices is the tokens; a count-aggregated meter
// ignores the value and counts the row.
//
// It records the quantity whatever the charge: per-token pricing makes most
// single calls sub-cent, and a meter is for accumulating small quantities so they
// can be priced in bulk at the end of the period.
//
// One act, one row: [engine.IngestUsageEvent] already dedups on the idempotency
// key, so a retry of the same requestId counts the tokens once. This used to be a
// hand-rolled second copy of that function without the dedup.
func meterUsage(db *datastore.Datastore, req usageRequest) {
	if req.TotalTokens <= 0 && req.AmountMicros <= 0 && req.Amount <= 0 {
		return // nothing happened worth metering
	}
	rootKey := db.NewKey("synckey", "", 1, nil)
	meters := make([]*meter.Meter, 0, 1)
	q := meter.Query(db).Ancestor(rootKey).Filter("EventName=", "api-usage").Limit(1)
	if _, err := q.GetAll(&meters); err != nil || len(meters) == 0 {
		return
	}
	if _, _, err := engine.IngestUsageEvent(db, meters[0].Id(), req.User,
		int64(req.TotalTokens), req.RequestID, time.Now(),
		map[string]any{"model": req.Model, "provider": req.Provider},
	); err != nil {
		log.Error("Failed to record usage meter event: %v", err)
	}
}

// GetUsage returns usage transactions for an IAM user, filtered by tag "api-usage".
//
//	GET /v1/billing/usage?user=hanzo/alice&currency=usd
func GetUsage(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	ctx := org.Namespaced(c.Context())
	db := datastore.New(ctx)

	user := strings.TrimSpace(c.Query("user"))
	if user == "" {
		return http.Fail(c, 400, "user query parameter is required", nil)
	}

	rootKey := db.NewKey("synckey", "", 1, nil)

	transs := make([]*transaction.Transaction, 0)
	q := transaction.Query(db).Ancestor(rootKey).
		Filter("Test=", org.TestMode()).
		Filter("SourceKind=", "iam-user").
		Filter("SourceId=", user).
		Filter("Tags=", "api-usage")

	cur := currency.Type(strings.ToLower(c.Query("currency")))
	if cur != "" {
		q = q.Filter("Currency=", cur)
	}

	if _, err := q.GetAll(&transs); err != nil {
		log.Error("Failed to query usage transactions: %v", err, c)
		return http.Fail(c, 500, "failed to query usage", err)
	}

	items := make([]map[string]any, 0, len(transs))
	for _, t := range transs {
		items = append(items, map[string]any{
			"transactionId": t.Id(),
			"amount":        t.Amount,
			"currency":      t.Currency,
			"notes":         t.Notes,
			"metadata":      t.Metadata,
			"createdAt":     t.CreatedAt,
		})
	}

	return c.JSON(200, map[string]any{
		"user":  user,
		"count": len(items),
		"usage": items,
	})
}

// paidBySources is what may pay for a usage call. Empty is "not stated".
var paidBySources = map[string]bool{"plan": true, "prepaid": true, "credits": true, "line": true, "hanzo": true}

// usageRefused is a usage write refused for what the CALLER sent (400).
type usageRefused struct{ msg string }

func (e usageRefused) Error() string { return e.msg }

var (
	// errUsageInFlight is a concurrent write under the same idempotency key (409).
	errUsageInFlight = errors.New("usage recording already in progress")
	// errUsageUncounted is a store that cannot keep the exact running total (503).
	errUsageUncounted = errors.New("this store cannot keep an exact usage total")
)

// usageCounter names the running micro-dollar total one subject's usage debits
// against: per org, mode, subject and currency. The name carries the org because
// a shared Postgres keeps one counter table for every tenant.
func usageCounter(org string, test bool, subject string, cur currency.Type) string {
	mode := "live"
	if test {
		mode = "test"
	}
	return "usage-micros:" + org + ":" + mode + ":" + subject + ":" + string(cur)
}

// usageCents is what one usage act debits, in whole cents, so that the cents
// debited across all of a subject's usage equal floor(total micros / 10_000)
// EXACTLY. The act's micros land on one atomic running total and the act debits
// the cent boundaries its own span of that total crosses: a sub-cent call debits
// 0 and carries its micros forward, the call that completes the cent debits it.
// Nothing is rounded away and no cent is debited twice, under any concurrency,
// because the running total moves in one storage statement (db.Counter).
//
// It returns the counter so a caller whose row then fails to write can take its
// micros back out.
func usageCents(ctx context.Context, d *datastore.Datastore, name string, micros int64) (int64, db.Counter, error) {
	counter, ok := d.DB().(db.Counter)
	if !ok {
		return 0, nil, errUsageUncounted
	}
	if micros == 0 {
		return 0, counter, nil
	}
	total, err := counter.Add(ctx, name, micros)
	if err != nil {
		return 0, nil, err
	}
	return floorCents(total) - floorCents(total-micros), counter, nil
}

// floorCents is whole cents in a micro-dollar total, rounding toward -inf.
func floorCents(micros int64) int64 {
	c := micros / 10000
	if micros%10000 < 0 {
		c--
	}
	return c
}

// RecordUsage records an API usage event as a Withdraw transaction.
//
//	POST /v1/billing/usage
//
// Every served call writes a row, a $0 call and a sub-cent call included: the
// row carries the call's model, tokens, scope, our cost and what paid, and debits
// the whole cents the call's micros complete (usageCents).
func RecordUsage(c *zip.Ctx) error {
	var req usageRequest
	if err := c.Bind(&req); err != nil {
		return http.Fail(c, 400, "invalid request body", err)
	}
	idemKey := strings.TrimSpace(c.Header("X-Idempotency-Key"))
	if idemKey == "" {
		idemKey = strings.TrimSpace(req.RequestID)
	}
	out, replay, err := writeUsage(c, req, idemKey)
	var refused usageRefused
	switch {
	case errors.As(err, &refused):
		return http.Fail(c, 400, refused.msg, nil)
	case errors.Is(err, errUsageInFlight):
		return http.Fail(c, 409, err.Error(), nil)
	case errors.Is(err, errUsageUncounted):
		return http.Fail(c, 503, "usage cannot be counted exactly on this store", err)
	case err != nil:
		return http.Fail(c, 500, "failed to record usage", err)
	case replay != nil:
		c.SetHeader("Content-Type", "application/json")
		return c.Bytes(200, replay)
	}
	return c.JSON(201, out)
}

// writeUsage is the ONE usage write, behind POST /v1/billing/usage and ZAP
// billing.recordUsage alike. It returns the written row's answer, or the stored
// answer of a completed retry under the same idempotency key.
func writeUsage(c *zip.Ctx, req usageRequest, idemKey string) (map[string]any, []byte, error) {
	org := middleware.GetOrganization(c)
	d := datastore.New(org.Namespaced(c.Context()))

	req.User = strings.TrimSpace(req.User)
	if req.User == "" {
		return nil, nil, usageRefused{"user is required"}
	}
	if req.Amount < 0 || req.AmountMicros < 0 || req.CostMicros < 0 {
		return nil, nil, usageRefused{"amount, amountMicros and costMicros must not be negative"}
	}
	req.PaidBy = strings.ToLower(strings.TrimSpace(req.PaidBy))
	if req.PaidBy != "" && !paidBySources[req.PaidBy] {
		return nil, nil, usageRefused{"paidBy must be one of plan, prepaid, credits, line, hanzo"}
	}

	// THE QUANTITY IS NOT THE MONEY, and it is recorded before the money is. See
	// meterUsage.
	go meterUsage(d, req)

	// `amountMicros` (micro-USD, 1e6=$1) carries full precision; `amount` (cents)
	// is the back-compat form of the same charge.
	micros := req.AmountMicros
	if micros == 0 {
		micros = req.Amount * 10000 // 1 cent = 10_000 micro-USD
	}

	// Idempotency guard (money-critical). A retry (client lost the response) or a
	// double-submit MUST create AT MOST ONE row. Keyed on the caller's
	// X-Idempotency-Key, else the requestId the caller sends per spend; the
	// datastore is org-namespaced, so the key is per-tenant. No key => the caller
	// opted out of the guard.
	var idemRec *idempotencykey.IdempotencyKey
	if idemKey != "" {
		rec, replay, gerr := idempotencykey.Begin(d, "billing-usage", idemKey)
		switch {
		case gerr != nil:
			// Guard store unavailable — proceed WITHOUT the replay guard rather
			// than drop a legitimate usage record (matches topup posture).
			log.Error("usage idempotency Begin failed (org=%s key=%s): %v", org.Name, idemKey, gerr, c)
		case replay:
			if rec.Status == idempotencykey.StatusCompleted && rec.Response != "" {
				return nil, []byte(rec.Response), nil
			}
			return nil, nil, errUsageInFlight
		default:
			idemRec = rec
		}
	}
	release := func() {
		if idemRec != nil {
			_ = idemRec.Delete()
		}
	}

	cur := currency.Type(strings.ToLower(req.Currency))
	if cur == "" {
		cur = "usd"
	}

	counter := usageCounter(org.Name, org.TestMode(), req.User, cur)
	amountCents, total, err := usageCents(c.Context(), d, counter, micros)
	if err != nil {
		release()
		return nil, nil, err
	}

	trans := transaction.New(d)
	trans.Type = transaction.Withdraw
	trans.SourceId = req.User
	trans.SourceKind = "iam-user"
	trans.Currency = cur
	trans.Amount = currency.Cents(amountCents)
	trans.Notes = fmt.Sprintf("API usage: %s (%d tokens)", req.Model, req.TotalTokens)
	trans.Tags = "api-usage"
	// Scope attribution (issue #70): the indexed dimensions the per-scope spend
	// cap sums over. Normalized so the default project ("default") and "no
	// project" collapse to the same "" scope the org-wide cap counts.
	trans.Project = spendalert.NormalizeProject(req.Project)
	trans.Service = strings.TrimSpace(req.Service)
	trans.Metadata = Map{
		"model":            req.Model,
		"provider":         req.Provider,
		"promptTokens":     req.PromptTokens,
		"completionTokens": req.CompletionTokens,
		"totalTokens":      req.TotalTokens,
		// The exact charge and our exact cost, in micro-USD. sum(amountMicros) is
		// the exact spend; sum(amount) is floor(sum(amountMicros)/10_000) by
		// construction (usageCents).
		"amountMicros": micros,
		"costMicros":   req.CostMicros,
		"paidBy":       req.PaidBy,
		"requestId":    req.RequestID,
		"premium":      req.Premium,
		"stream":       req.Stream,
		"status":       req.Status,
		"clientIp":     req.ClientIP,
	}
	if org.TestMode() {
		trans.Test = true
	}

	// The call already happened, so its row is written whatever the balance:
	// balance gating happens at request time, before the call is served.
	if err := trans.Create(); err != nil {
		// No row holds these micros, so the running total gives them back.
		if micros != 0 {
			if _, rerr := total.Add(context.WithoutCancel(c.Context()), counter, -micros); rerr != nil {
				log.Error("RECONCILE: usage counter %s holds %d micros no row records: %v", counter, micros, rerr, c)
			}
		}
		release()
		log.Error("Failed to record usage transaction: %v", err, c)
		return nil, nil, err
	}

	if amountCents > 0 {
		// Referral revenue share and the OSS-developer payout accrue on money
		// actually debited. Fire-and-forget: neither may fail the usage write.
		go engine.TrackRevenueShare(d, req.User, currency.Cents(amountCents), cur, trans.Id(), org.TestMode())
		go engine.AccrueOSSPayout(d, org.Name, req.User, currency.Cents(amountCents), cur, trans.Id(), !org.Live)
	}

	// Emit the api-usage debit to the analytics collector (commerce.events) so the
	// fleet usage view (admin.hanzo.ai) can aggregate metered spend — best-effort,
	// fire-and-forget, never blocks the money path.
	emitAPIUsageDebit(c, org.Name, &req, amountCents, micros)

	// Fire any spend-alert this debit pushed over its soft-warn threshold or cap —
	// the "alert" half of a spend-alert (the "cap" half is the metering gate). This
	// is the ONLY write path where period spend accrues, so the crossing is detected
	// here, once, off the money path and debounced per (period, level).
	ev, _ := c.Locals("events").(*events.Client)
	go checkAndFireSpendAlerts(context.WithoutCancel(c.Context()), d, org.Name, org.TestMode(),
		spendalert.NormalizeProject(req.Project), strings.TrimSpace(req.Service), ev)

	resp := map[string]any{
		"transactionId": trans.Id(),
		"user":          req.User,
		"amount":        amountCents,
		"amountMicros":  micros,
		"costMicros":    req.CostMicros,
		"paidBy":        req.PaidBy,
		"currency":      cur,
		"type":          "withdraw",
	}

	// Seal the idempotency guard with the exact success body so a retry replays
	// it verbatim (no second row, identical response).
	if idemRec != nil {
		if body, mErr := json.Marshal(resp); mErr == nil {
			_ = idempotencykey.Complete(idemRec, string(body))
		}
	}
	return resp, nil, nil
}

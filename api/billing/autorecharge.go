package billing

import (
	"context"
	stdjson "encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/events"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/autorecharge"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/paymentmethod"
	"github.com/hanzoai/commerce/models/transaction/util"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/thirdparty/kms"
	jsonhttp "github.com/hanzoai/commerce/util/json/http"
)

// loadAutoRecharge returns the org's auto-recharge config (keyed by the org
// slug) or nil if none exists.
func loadAutoRecharge(db *datastore.Datastore, userId string) *autorecharge.AutoRecharge {
	rootKey := db.NewKey("synckey", "", 1, nil)
	cfgs := make([]*autorecharge.AutoRecharge, 0, 1)
	keys, err := autorecharge.Query(db).Ancestor(rootKey).Filter("UserId=", userId).GetAll(&cfgs)
	if err != nil || len(cfgs) == 0 || len(keys) == 0 {
		return nil
	}
	// A query-loaded entity carries neither its datastore nor its key. Every
	// caller saves what this returns, so the binding belongs here, once. Rebind,
	// not Init: Init would re-apply Defaults() over the values just loaded. And
	// the KEY, with its parent: without it a save wrote a second, parentless row
	// this ancestor query never reads, so it answered success and the rule on
	// file did not change.
	cfg := cfgs[0]
	cfg.Rebind(db)
	if err := cfg.SetKey(db.NewKey(keys[0].Kind(), keys[0].StringID(), keys[0].IntID(), rootKey)); err != nil {
		return nil
	}
	reconcile(db, cfg, keys[0])
	return cfg
}

// reconcile folds a parentless row back into the rule on file and removes it.
//
// Such a row is what every save of an existing rule wrote before the key was
// bound: an edit, and the sweep's stamp after a reload. Every read showed the
// rule on file, so that is what the customer last saw and what is kept — with
// one exception, in the direction that can charge nobody: a stray that switched
// the rule off was the customer's word, and an armed rule on file is switched
// off. A stray never arms a rule or raises what it charges; that would be an
// off-session charge on consent nobody saw recorded. The strays are deleted
// either way, so this runs once per org.
func reconcile(db *datastore.Datastore, cfg *autorecharge.AutoRecharge, own datastore.Key) {
	rows := make([]*autorecharge.AutoRecharge, 0, 2)
	keys, err := autorecharge.Query(db).Filter("UserId=", cfg.UserId).GetAll(&rows)
	if err != nil || len(rows) < 2 || len(keys) != len(rows) {
		return
	}
	var latest *autorecharge.AutoRecharge
	strays := make([]datastore.Key, 0, 1)
	for i, row := range rows {
		if keys[i].IntID() == own.IntID() && keys[i].StringID() == own.StringID() {
			continue
		}
		strays = append(strays, keys[i])
		if latest == nil || row.UpdatedAt.After(latest.UpdatedAt) {
			latest = row
		}
	}
	if latest == nil {
		return
	}
	if cfg.Enabled && !latest.Enabled {
		log.Info("auto-recharge: %q was switched off by an edit that never reached its rule; switching it off", cfg.UserId)
		cfg.Enabled = false
		// Kept until the switch-off is on file, so a write that lost is tried
		// again on the next read rather than dropped.
		if ok, err := cfg.Swap(); err != nil || !ok {
			log.Error("auto-recharge: could not switch off %q (wrote=%v): %v", cfg.UserId, ok, err)
			return
		}
	}
	for _, k := range strays {
		if err := db.Delete(k); err != nil {
			log.Error("auto-recharge: could not remove a stray rule row for %q: %v", cfg.UserId, err)
		}
	}
}

// changeRule applies change to the org's rule under compare-and-swap. It reads
// the rule again on every attempt, so the sweep and an admin's save never write
// over one another: whichever writes second reads what the first wrote. change
// answers false to write nothing. A nil rule is an org with none.
func changeRule(db *datastore.Datastore, org string, change func(*autorecharge.AutoRecharge) bool) (*autorecharge.AutoRecharge, bool, error) {
	for range mixin.Swaps {
		cfg := loadAutoRecharge(db, org)
		if cfg == nil {
			return nil, false, nil
		}
		if !change(cfg) {
			return cfg, false, nil
		}
		ok, err := cfg.Swap()
		if err != nil {
			return nil, false, err
		}
		if ok {
			return cfg, true, nil
		}
	}
	return nil, false, fmt.Errorf("auto-recharge rule for %s changed under %d reads in a row", org, mixin.Swaps)
}

// defaultPaymentMethod returns the customer's default payment method, or nil.
func defaultPaymentMethod(db *datastore.Datastore, customerId string) *paymentmethod.PaymentMethod {
	rootKey := db.NewKey("synckey", "", 1, nil)
	pms := make([]*paymentmethod.PaymentMethod, 0, 1)
	if _, err := paymentmethod.Query(db).Ancestor(rootKey).
		Filter("CustomerId=", customerId).
		Filter("IsDefault=", true).GetAll(&pms); err != nil {
		return nil
	}
	if len(pms) == 0 {
		return nil
	}
	return pms[0]
}

// AutoRecharge is one org's auto-recharge rule as its readers see it: whether it
// fires, the balance it fires below, the amount it charges, and when it last fired.
//
// Stored is not on the wire — it records whether a ROW exists. An org that never
// set the rule reads as the disabled one, which is exactly how the sweep already
// treats it, so the values alone cannot tell "never configured" from "configured
// and switched off". Anything that must tell those apart asks here rather than
// inferring it from a zero, which would make the two indistinguishable the day a
// customer legitimately sets an amount of nothing.
type AutoRecharge struct {
	Subject         string `json:"subject"`
	Enabled         bool   `json:"enabled"`
	ThresholdCents  int64  `json:"thresholdCents"`
	AmountCents     int64  `json:"amountCents"`
	Currency        string `json:"currency"`
	LastRechargedAt string `json:"lastRechargedAt"`
	// PerDay is the most reloads the sweep makes in one UTC day (ReloadsPerDay).
	PerDay int `json:"perDay"`

	Stored bool `json:"-"`
}

// AutoRechargeEdit is what a caller may change about the rule. Every field is a
// plain value, not a pointer: a write states the WHOLE rule, because there is one
// small row and no field whose absence could mean anything but its zero.
type AutoRechargeEdit struct {
	Enabled        bool   `json:"enabled"`
	ThresholdCents int64  `json:"thresholdCents"`
	AmountCents    int64  `json:"amountCents"`
	Currency       string `json:"currency,omitempty"`
}

// ReloadsPerDay is how many times the sweep asks the processor to charge one
// org's card in a UTC day.
//
// The sweep runs every 15 minutes, so a balance that drains as fast as it is
// refilled — a leaked key, a runaway agent — would otherwise be reloaded 96 times
// a day, each one an off-session charge nobody is watching. Three bounds the worst
// day at three times the amount the org's admin chose; past it the balance runs
// down to the spend gate and a person adds funds by hand. A decline counts too, so
// a failing card is not tried again all day.
const ReloadsPerDay = 3

// ErrNoDefaultPaymentMethod is the refusal of a rule armed with no card to charge.
// Its text is the customer's sentence, and a client matches on it.
var ErrNoDefaultPaymentMethod = errors.New("a default payment method is required to enable auto-recharge")

// ruleRefusal is a rule value its caller can fix, in the customer's words.
type ruleRefusal struct{ text string }

func (r ruleRefusal) Error() string { return r.text }

// IsAutoRechargeRefused reports a rule WriteAutoRecharge refused for a value its
// caller can fix — an amount or threshold out of range, or no card on file — as
// distinct from a store that failed us. The message is the customer's sentence.
//
// The refusals bite only when the rule is ARMED: a disabled rule charges nobody,
// so a zero amount or a card not yet on file is a customer part-way through
// setting up, and an old rule out of range can always be switched off.
func IsAutoRechargeRefused(err error) bool {
	var r ruleRefusal
	return errors.As(err, &r) || errors.Is(err, ErrNoDefaultPaymentMethod)
}

// checkRule refuses an armed rule whose amount the charge would refuse, or whose
// threshold is outside the same range. The bound is the top-up's (topupBounds),
// which chargeAndCredit enforces on every reload: one bound, one definition, so a
// rule cannot be stored that its own sweep would refuse to charge.
func checkRule(in AutoRechargeEdit) error {
	lo, hi := topupBounds()
	if in.AmountCents < lo || in.AmountCents > hi {
		return ruleRefusal{fmt.Sprintf("The reload amount must be between %s and %s.", usd(lo), usd(hi))}
	}
	if in.ThresholdCents < 0 || in.ThresholdCents > hi {
		return ruleRefusal{fmt.Sprintf("The balance to reload below must be between %s and %s.", usd(0), usd(hi))}
	}
	return nil
}

// usd renders whole cents as dollars with thousands separators: 500000 → "$5,000".
func usd(cents int64) string {
	d := strconv.FormatInt(cents/100, 10)
	for i := len(d) - 3; i > 0; i -= 3 {
		d = d[:i] + "," + d[i:]
	}
	if c := cents % 100; c != 0 {
		return fmt.Sprintf("$%s.%02d", d, c)
	}
	return "$" + d
}

// autoRechargeView renders a loaded row — or the absence of one — as the rule its
// readers see. An org with no row has the DISABLED rule in usd, which is the rule
// the sweep reads it as; that agreement is the point, since a reader shown one
// default and a sweep applying another is two rules again.
func autoRechargeView(cfg *autorecharge.AutoRecharge, subject string) AutoRecharge {
	if cfg == nil {
		return AutoRecharge{Subject: subject, Currency: "usd", PerDay: ReloadsPerDay}
	}
	return AutoRecharge{
		Subject:         cfg.UserId,
		Enabled:         cfg.Enabled,
		ThresholdCents:  cfg.ThresholdCents,
		AmountCents:     cfg.AmountCents,
		Currency:        cfg.Currency,
		LastRechargedAt: cfg.LastRechargedAt,
		PerDay:          ReloadsPerDay,
		Stored:          true,
	}
}

// autoRechargeResponse is this endpoint's wire, and stays a map on purpose: map
// keys are emitted in sorted order, so serialising the view struct instead would
// reorder every field of a response clients have parsed since it shipped.
//
// The subject rides as "userId" — the name the rule was keyed under before the
// key became the org — and lastRechargedAt is present for a stored row even when
// it is empty, absent entirely when there is no row. That is what Stored buys:
// the quirk is a question about the ROW, and a view that only carried values
// could not answer it.
func autoRechargeResponse(v AutoRecharge) map[string]any {
	out := map[string]any{
		"userId":         v.Subject,
		"enabled":        v.Enabled,
		"thresholdCents": v.ThresholdCents,
		"amountCents":    v.AmountCents,
		"currency":       v.Currency,
		"perDay":         v.PerDay,
	}
	if v.Stored {
		out["lastRechargedAt"] = v.LastRechargedAt
	}
	return out
}

// ReadAutoRecharge is the org's auto-recharge rule — the QUERY, with no HTTP in it.
//
// It takes values rather than a request so a caller that is not a request can ask.
// The console's auto-reload switch, the sweep that fires the rule and a peer that
// holds no datastore all need the same answer, and a peer re-deriving the lookup
// would be a second statement of one money rule — which is two rules, drifting
// from the day the second one is written.
func ReadAutoRecharge(ctx context.Context, org *organization.Organization) (AutoRecharge, error) {
	if org == nil {
		return AutoRecharge{}, errors.New("auto-recharge: no organization")
	}
	db := datastore.New(org.Namespaced(ctx))
	return autoRechargeView(loadAutoRecharge(db, org.Name), org.Name), nil
}

// WriteAutoRecharge sets the org's auto-recharge rule — the WRITE, with no HTTP in
// it. It upserts: there is one rule per org, so a second write edits the first.
//
// It takes values rather than a request for the reason ReadAutoRecharge does, and
// more sharply: this is the write that decides when an off-session card charge
// fires, so the endpoint and any peer must arm it under the SAME conditions. A
// peer with its own copy of "you need a card on file first" is a peer that can
// arm a rule this one would refuse.
//
// A refusal of the caller's own values answers IsAutoRechargeRefused; anything
// else is the store failing us. Which status each becomes is the endpoint's
// business, not this one's.
func WriteAutoRecharge(ctx context.Context, org *organization.Organization, in AutoRechargeEdit) (AutoRecharge, error) {
	if org == nil {
		return AutoRecharge{}, errors.New("auto-recharge: no organization")
	}
	db := datastore.New(org.Namespaced(ctx))

	if in.Enabled {
		if err := checkRule(in); err != nil {
			return AutoRecharge{}, err
		}
		if defaultPaymentMethod(db, org.Name) == nil {
			return AutoRecharge{}, ErrNoDefaultPaymentMethod
		}
	}

	cur := strings.ToLower(strings.TrimSpace(in.Currency))
	if cur == "" {
		cur = "usd"
	}

	// The rule's own fields, and nothing the sweep keeps beside them (the day's
	// reload count): a save that wrote a stale count back would reopen the limit.
	set := func(c *autorecharge.AutoRecharge) bool {
		c.Enabled, c.ThresholdCents, c.AmountCents, c.Currency = in.Enabled, in.ThresholdCents, in.AmountCents, cur
		return true
	}
	cfg, _, err := changeRule(db, org.Name, set)
	if err != nil {
		return AutoRecharge{}, err
	}
	if cfg == nil {
		cfg = autorecharge.New(db)
		cfg.UserId = org.Name
		set(cfg)
		if err := cfg.Create(); err != nil {
			return AutoRecharge{}, err
		}
	}

	return autoRechargeView(cfg, org.Name), nil
}

// GetAutoRecharge returns the org's auto-recharge config (or disabled defaults).
//
//	GET /v1/billing/recharge
func GetAutoRecharge(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	v, err := ReadAutoRecharge(c.Context(), org)
	if err != nil {
		log.Error("Failed to read auto-recharge config for org %q: %v", org.Name, err, c)
		return jsonhttp.Fail(c, 500, "failed to read auto-recharge config", err)
	}
	b, _ := stdjson.Marshal(autoRechargeResponse(v))
	c.SetHeader("Content-Type", "application/json")
	return c.Bytes(200, b)
}

// SetAutoRecharge upserts the org's auto-recharge config.
//
//	PUT /v1/billing/recharge
//
// Enabling requires a default payment method on file (the card that will be
// charged off-session when the balance runs low).
//
// The rule charges the ORG's card, so it is the org's admins' to set — and a
// platform SuperAdmin's or a service's (mayActForOrg). A member is told so.
func SetAutoRecharge(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	if !mayActForOrg(c) {
		return jsonhttp.Fail(c, 403, askAnAdmin(c, "change its auto-reload"), nil)
	}

	var in AutoRechargeEdit
	if err := c.Bind(&in); err != nil {
		return jsonhttp.Fail(c, 400, "invalid request body", err)
	}

	v, err := WriteAutoRecharge(c.Context(), org, in)
	if err != nil {
		if IsAutoRechargeRefused(err) {
			return jsonhttp.Fail(c, 400, err.Error(), nil)
		}
		log.Error("Failed to save auto-recharge config for org %q: %v", org.Name, err, c)
		return jsonhttp.Fail(c, 500, "failed to save auto-recharge config", err)
	}

	return c.JSON(200, autoRechargeResponse(v))
}

// RechargeResult is one organization's outcome in a sweep — charged, skipped,
// or failed with the reason. The amount, balance and transaction are absent on
// anything but a charge, which is exactly what omitempty says here: a skipped
// org has no amount, rather than an amount of zero.
type RechargeResult struct {
	OrgName       string `json:"orgName"`
	UserId        string `json:"userId"`
	Charged       bool   `json:"charged"`
	AmountCents   int64  `json:"amountCents,omitempty"`
	Currency      string `json:"currency,omitempty"`
	Test          bool   `json:"test,omitempty"` // charged the org's sandbox, so no sale
	BalanceCents  int64  `json:"balanceCents,omitempty"`
	TransactionId string `json:"transactionId,omitempty"`
	Error         string `json:"error,omitempty"`
}

// RechargeRun is one whole sweep: how many organizations were considered, how
// many were charged, and what happened to each one that was not simply skipped.
// Orgs is the population, not the row count — that difference is how a reader
// knows a quiet run swept anything at all.
type RechargeRun struct {
	Orgs    int              `json:"orgs"`
	Charged int              `json:"charged"`
	Results []RechargeResult `json:"results"`
}

// RunAutoRecharge sweeps every organization and, for those with auto-recharge
// enabled whose available balance has fallen below the threshold, charges the
// default card and credits the balance.
//
// It takes values because the caller is a SCHEDULE, not a person: there is no
// request behind an off-session charge, and the one thing that must never be
// read from one is the retry key. A single run-all request's header would be
// shared by every org this loop charges, which is why each recharge derives its
// own key from its own identity instead.
//
// A failure to charge one org is that org's result, not the sweep's: the loop
// continues, so one bad card cannot leave every later customer unfunded. Only
// the population read failing ends the run.
func RunAutoRecharge(ctx context.Context, kmsClient *kms.CachedClient, ev *events.Client) (*RechargeRun, error) {
	rootDb := datastore.New(ctx)
	orgs := make([]*organization.Organization, 0)
	if _, err := organization.Query(rootDb).GetAll(&orgs); err != nil {
		return nil, err
	}

	run := &RechargeRun{Orgs: len(orgs), Results: make([]RechargeResult, 0)}

	for _, org := range orgs {
		db := datastore.New(org.Namespaced(ctx))

		cfg := loadAutoRecharge(db, org.Name)
		if cfg == nil || !cfg.Enabled || cfg.AmountCents <= 0 {
			continue
		}

		cur := currency.Type(strings.ToLower(cfg.Currency))
		if cur == "" {
			cur = "usd"
		}

		// Available = balance - holds. Skip if still above the threshold.
		var available int64
		if datas, err := util.GetTransactionsByCurrency(org.Namespaced(ctx), org.Name, "iam-user", cur, org.TestMode()); err == nil {
			if data, ok := datas.Data[cur]; ok {
				available = int64(data.Balance) - int64(data.Holds)
			}
		}
		if available >= cfg.ThresholdCents {
			continue
		}

		// The reload is RESERVED before any card is touched: the day's count is
		// taken under compare-and-swap against the rule as it stands now, so a
		// rule switched off or changed since it was read above is not charged, a
		// second sweep cannot take the same last reload, and the limit holds
		// however many sweeps overlap. A rule that has reloaded ReloadsPerDay
		// times today waits for tomorrow, and says so in its row.
		today := time.Now().UTC().Format(time.DateOnly)
		full := false
		cfg, reserved, rerr := changeRule(db, org.Name, func(c *autorecharge.AutoRecharge) bool {
			if !c.Enabled || c.AmountCents <= 0 || available >= c.ThresholdCents {
				return false
			}
			if c.Day == today && c.Reloads >= ReloadsPerDay {
				full = true
				return false
			}
			if c.Day != today {
				c.Day, c.Reloads, c.Txn = today, 0, ""
			}
			c.Reloads++
			return true
		})
		switch {
		case rerr != nil:
			run.Results = append(run.Results, RechargeResult{
				OrgName: org.Name, UserId: org.Name, Charged: false, Error: rerr.Error(),
			})
			continue
		case full:
			run.Results = append(run.Results, RechargeResult{
				OrgName: org.Name, UserId: org.Name, Charged: false,
				Error: fmt.Sprintf("auto-reload already charged %d times today", ReloadsPerDay),
			})
			continue
		case !reserved:
			continue
		}
		cur = currency.Type(strings.ToLower(cfg.Currency))
		if cur == "" {
			cur = "usd"
		}

		pm := defaultPaymentMethod(db, org.Name)
		if pm == nil {
			release(db, org.Name, today)
			run.Results = append(run.Results, RechargeResult{
				OrgName: org.Name, UserId: org.Name, Charged: false,
				Error: "no default payment method",
			})
			continue
		}

		// Hydrate this org's payment credentials so ProcessorsForOrg sees real
		// Square creds for the off-session charge.
		if kmsClient != nil {
			if err := kms.Hydrate(kmsClient, org); err != nil {
				log.Error("KMS hydration failed for org %q during auto-recharge: %v", org.Name, err)
			}
		}

		// The cron is not a client request, so it has NO X-Idempotency-Key to read
		// — and reading the run-all request's header would be actively wrong, since
		// one header would then be shared by every org this loop charges. The key
		// is derived from the recharge's own identity (org, amount, currency) in a
		// coarse window: an overlapping or re-fired run collapses onto the same key
		// and charges once, while a genuine later recharge — the balance fell below
		// the threshold again — gets a fresh key and proceeds.
		guard := windowKey("recharge:" + org.Name + ":amount:" +
			strconv.FormatInt(cfg.AmountCents, 10) + ":cur:" + string(cur))

		desc := fmt.Sprintf("Auto-recharge %d %s for %s", cfg.AmountCents, cur, org.Name)
		txID, balanceCents, err := chargeAndCredit(ctx, ev, org, db, pm, cfg.AmountCents, cur, org.Name, guard, desc)
		if err != nil {
			// A charge refused before the processor was asked gives its
			// reservation back. Every other one keeps it: a decline is an attempt
			// on the card, and an answer lost or a credit failed may hide money
			// that moved, so the limit counts the attempts the processor saw.
			if unasked(err) {
				release(db, org.Name, today)
			}
			log.Error("Auto-recharge charge failed for org %q: %v", org.Name, err)
			run.Results = append(run.Results, RechargeResult{
				OrgName: org.Name, UserId: org.Name, Charged: false, Error: err.Error(),
			})
			continue
		}

		// The reload is recorded on the rule as it stands now. A charge the
		// processor answered by replaying one already counted — a re-fired sweep
		// inside the same idempotency window — gives its reservation back.
		now := time.Now().UTC().Format(time.RFC3339)
		if _, _, serr := changeRule(db, org.Name, func(c *autorecharge.AutoRecharge) bool {
			if txID != "" && txID == c.Txn {
				if c.Day == today && c.Reloads > 0 {
					c.Reloads--
				}
			} else {
				c.Txn = txID
			}
			c.LastRechargedAt = now
			return true
		}); serr != nil {
			log.Error("auto-recharge: could not record the reload for %q: %v", org.Name, serr)
		}

		run.Charged++
		run.Results = append(run.Results, RechargeResult{
			OrgName:       org.Name,
			UserId:        org.Name,
			Charged:       true,
			AmountCents:   cfg.AmountCents,
			Currency:      string(cur),
			Test:          org.TestMode(),
			BalanceCents:  int64(balanceCents),
			TransactionId: txID,
		})
	}

	return run, nil
}

// release gives back a reload reserved today that charged nothing.
func release(db *datastore.Datastore, org, today string) {
	if _, _, err := changeRule(db, org, func(c *autorecharge.AutoRecharge) bool {
		if c.Day != today || c.Reloads == 0 {
			return false
		}
		c.Reloads--
		return true
	}); err != nil {
		log.Error("auto-recharge: could not release a reload for %q: %v", org, err)
	}
}

// RunAutoRechargeAllOrgs iterates every organization and, for those with
// auto-recharge enabled whose available balance is below the threshold, charges
// the default payment method and credits the balance. Intended to be invoked on
// a recurring schedule (CronJob) by the platform.
//
//	POST /v1/billing/recharge/run-all
func RunAutoRechargeAllOrgs(c *zip.Ctx) error {
	run, err := RunAutoRecharge(c.Context(), kmsOf(c), eventsOf(c))
	if err != nil {
		log.Error("Failed to list organizations for auto-recharge: %v", err, c)
		return jsonhttp.Fail(c, 500, "failed to list organizations", err)
	}
	return c.JSON(200, run)
}

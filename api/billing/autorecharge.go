package billing

import (
	"cmp"
	"context"
	stdjson "encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/billing/creditledger"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/events"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/autorecharge"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/paymentmethod"
	"github.com/hanzoai/commerce/models/paymentorg"
	"github.com/hanzoai/commerce/models/plan"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/models/types/currency"
	jsonhttp "github.com/hanzoai/commerce/util/json/http"
	"github.com/hanzoai/commerce/util/nscontext"
)

// loadAutoRecharge returns the payer's auto-recharge rule (keyed by the payer's
// subject) or nil if none exists.
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

// changeRule applies change to the payer's rule under compare-and-swap. It reads
// the rule again on every attempt, so the sweep and an admin's save never write
// over one another: whichever writes second reads what the first wrote. change
// answers false to write nothing. A nil rule is a payer with none.
func changeRule(db *datastore.Datastore, payer string, change func(*autorecharge.AutoRecharge) bool) (*autorecharge.AutoRecharge, bool, error) {
	for range mixin.Swaps {
		cfg := loadAutoRecharge(db, payer)
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
	return nil, false, fmt.Errorf("auto-recharge rule for %s changed under %d reads in a row", payer, mixin.Swaps)
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

// AutoRecharge is one payer's auto-recharge rule as its readers see it: whether it
// fires, the balance it fires below, the amount it charges, and when it last fired.
//
// Stored is not on the wire — it records whether a ROW exists. A payer that never
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
	// PerDay is the most reloads the sweep makes in one UTC day: the bound the
	// payer's plan states (reloadsFor), 0 when it is unbounded.
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
// org's card in a UTC day, for an org whose plan states no bound of its own
// (plan.Limits.ReloadsPerDay; reloadsFor).
//
// The sweep runs every 15 minutes, so a balance that drains as fast as it is
// refilled — a leaked key, a runaway agent — would otherwise be reloaded 96 times
// a day, each one an off-session charge nobody is watching. Three bounds the worst
// day at three times the amount the org's admin chose; past it the balance runs
// down to the spend gate and a person adds funds by hand. A decline counts too, so
// a failing card is not tried again all day.
const ReloadsPerDay = 3

// reloadsFor is the most reloads a day the sweep makes for payer: the bound the
// plan row payer is served from states — 0 for unbounded — and ReloadsPerDay when
// it states none or the payer holds no plan. db is the org's namespace.
func reloadsFor(ctx context.Context, db *datastore.Datastore, org *organization.Organization, payer string) int {
	if l := servedLimits(ctx, db, org, payer); l != nil && l.ReloadsPerDay != nil && *l.ReloadsPerDay >= 0 {
		return *l.ReloadsPerDay
	}
	return ReloadsPerDay
}

// servedLimits is the limits block of the plan authority row payer is served
// from, nil when it holds no plan or the row states none. db is the org's
// namespace.
func servedLimits(ctx context.Context, db *datastore.Datastore, org *organization.Organization, payer string) *plan.Limits {
	s := servedSubscription(db, payer, org.TestMode())
	if s == nil {
		return nil
	}
	return authorityLimits(ctx, servedAs(subscriptionSlug(s)))
}

// authorityLimits is the limits block of the plan authority row for slug, nil when
// there is no such row or it states none.
func authorityLimits(ctx context.Context, slug string) *plan.Limits {
	if slug == "" {
		return nil
	}
	row := plan.New(plan.AuthorityDB(ctx))
	if ok, err := row.Query().Filter("Slug=", slug).Get(); err != nil || !ok {
		return nil
	}
	return row.Limits
}

// payerOf is the payer a rule of org belongs to, named by subject: the org's own
// account for "" or the org slug, or an "<org>/<name>" account within the org — a
// person who pays from a wallet of their own (hanzoai/account). A subject outside
// the org is refused: one org's rules never address another's accounts.
func payerOf(org *organization.Organization, subject string) (string, error) {
	s := strings.ToLower(strings.TrimSpace(subject))
	own := strings.ToLower(strings.TrimSpace(org.Name))
	switch {
	case s == "" || s == own:
		return org.Name, nil
	case strings.HasPrefix(s, own+"/") && len(s) > len(own)+1:
		return s, nil
	}
	return "", fmt.Errorf("auto-recharge: %q is not an account of %s", subject, org.Name)
}

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
// threshold is outside the same range (CheckReload).
func checkRule(in AutoRechargeEdit) error {
	return reloadBounds(in.ThresholdCents, in.AmountCents)
}

// CheckReload is the one validator of a plan row's reload figures
// (plan.Limits.Reload), for any write of a plan: an amount of 0 states no reload
// and is valid; any other amount is the top-up's own range,  to ,000, and the
// threshold /usr/bin/zsh to ,000 (topupBounds) — the bound chargeAndCredit enforces on
// every reload, so a plan cannot state a reload its own sweep would refuse to
// charge. A refusal is the customer's sentence and answers IsAutoRechargeRefused.
func CheckReload(thresholdCents, amountCents int64) error {
	if amountCents == 0 {
		return nil
	}
	return reloadBounds(thresholdCents, amountCents)
}

// reloadBounds refuses an amount outside topupBounds, or a threshold outside /usr/bin/zsh
// to its top.
func reloadBounds(thresholdCents, amountCents int64) error {
	lo, hi := topupBounds()
	if amountCents < lo || amountCents > hi {
		return ruleRefusal{fmt.Sprintf("The reload amount must be between %s and %s.", usd(lo), usd(hi))}
	}
	if thresholdCents < 0 || thresholdCents > hi {
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
func autoRechargeView(cfg *autorecharge.AutoRecharge, subject string, perDay int) AutoRecharge {
	if cfg == nil {
		return AutoRecharge{Subject: subject, Currency: "usd", PerDay: perDay}
	}
	return AutoRecharge{
		Subject:         cfg.UserId,
		Enabled:         cfg.Enabled,
		ThresholdCents:  cfg.ThresholdCents,
		AmountCents:     cfg.AmountCents,
		Currency:        cfg.Currency,
		LastRechargedAt: cfg.LastRechargedAt,
		PerDay:          perDay,
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

// ReadAutoRecharge is a payer's auto-recharge rule — the QUERY, with no HTTP in it.
//
// subject names the payer (payerOf): "" or the org slug for the org's own account,
// "<org>/<name>" for a person in the org who pays from a wallet of their own.
//
// It takes values rather than a request so a caller that is not a request can ask.
// The console's auto-reload switch, the sweep that fires the rule and a peer that
// holds no datastore all need the same answer, and a peer re-deriving the lookup
// would be a second statement of one money rule — which is two rules, drifting
// from the day the second one is written.
func ReadAutoRecharge(ctx context.Context, org *organization.Organization, subject string) (AutoRecharge, error) {
	if org == nil {
		return AutoRecharge{}, errors.New("auto-recharge: no organization")
	}
	payer, err := payerOf(org, subject)
	if err != nil {
		return AutoRecharge{}, err
	}
	db := datastore.New(org.Namespaced(ctx))
	return autoRechargeView(loadAutoRecharge(db, payer), payer, reloadsFor(ctx, db, org, payer)), nil
}

// WriteAutoRecharge sets a payer's auto-recharge rule — the WRITE, with no HTTP in
// it. It upserts: there is one rule per payer, so a second write edits the first.
// subject names the payer as ReadAutoRecharge's does.
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
func WriteAutoRecharge(ctx context.Context, org *organization.Organization, subject string, in AutoRechargeEdit) (AutoRecharge, error) {
	if org == nil {
		return AutoRecharge{}, errors.New("auto-recharge: no organization")
	}
	payer, err := payerOf(org, subject)
	if err != nil {
		return AutoRecharge{}, err
	}
	if in.Enabled {
		if err := checkRule(in); err != nil {
			return AutoRecharge{}, err
		}
		if defaultPaymentMethod(datastore.New(org.Namespaced(ctx)), payer) == nil {
			return AutoRecharge{}, ErrNoDefaultPaymentMethod
		}
	}
	// The payer's own word: the rule is theirs from here on, whatever plan set it.
	v, _, err := writeRule(ctx, org, payer, func(c *autorecharge.AutoRecharge) bool {
		c.Enabled, c.ThresholdCents, c.AmountCents, c.Currency = in.Enabled, in.ThresholdCents, in.AmountCents, ruleCurrencyOf(in.Currency)
		c.Plan, c.Waiting = "", false
		return true
	})
	return v, err
}

// writeRule applies set to payer's rule under compare-and-swap, creating the rule
// when payer has none, and answers it and whether set wrote. set changes the rule's
// own fields and nothing the sweep keeps beside them (the day's count, the open
// trigger): a save that wrote a stale count back would reopen the limit.
func writeRule(ctx context.Context, org *organization.Organization, payer string, set func(*autorecharge.AutoRecharge) bool) (AutoRecharge, bool, error) {
	db := datastore.New(org.Namespaced(ctx))
	cfg, wrote, err := changeRule(db, payer, set)
	if err != nil {
		return AutoRecharge{}, false, err
	}
	if cfg == nil {
		cfg = autorecharge.New(db)
		cfg.UserId = payer
		if wrote = set(cfg); !wrote {
			return autoRechargeView(nil, payer, reloadsFor(ctx, db, org, payer)), false, nil
		}
		if err := cfg.Create(); err != nil {
			return AutoRecharge{}, false, err
		}
	}
	return autoRechargeView(cfg, payer, reloadsFor(ctx, db, org, payer)), wrote, nil
}

// ruleCurrencyOf is a rule's currency as stored: lower case, usd when unnamed.
func ruleCurrencyOf(cur string) string {
	if c := strings.ToLower(strings.TrimSpace(cur)); c != "" {
		return c
	}
	return "usd"
}

// Arming is what a purchase did to the buying payer's auto-recharge, answered for a
// plan whose row states a reload (plan.Limits.Reload). State is one of:
//
//	"armed"      the rule is on at the plan's figures
//	"kept"       the payer had switched its own rule on, and it is left as it stands
//	"needs_card" the payer has no card on file to charge: nothing was switched on,
//	             and saving a card switches it on (armOnCard)
type Arming struct {
	State string `json:"state"`
}

// The states an Arming answers.
const (
	armed     = "armed"
	kept      = "kept"
	needsCard = "needs_card"
)

// armForPlan switches payer's auto-recharge on at the figures the plan row for slug
// states (plan.Limits.Reload) once a purchase of the plan is paid, and answers what
// it did. A rule the payer switched on themselves is left as it stands (kept). A
// missing or switched-off rule, or one another plan switched on, takes the plan's
// figures and names the plan (armed) — which needs a card on file; with none the
// rule is marked waiting for one (needs_card). nil when the row states no reload,
// or when the rule could not be written for a reason the buyer cannot fix, which
// is logged. It never fails the purchase.
func armForPlan(ctx context.Context, org *organization.Organization, payer, slug string, cur currency.Type) *Arming {
	l := authorityLimits(ctx, slug)
	if l == nil || l.Reload == nil {
		return nil
	}
	in := AutoRechargeEdit{Enabled: true, ThresholdCents: l.Reload.ThresholdCents, AmountCents: l.Reload.AmountCents, Currency: string(cur)}
	if err := checkRule(in); err != nil {
		log.Error("auto-recharge: plan %q states a reload its sweep would refuse, so the rule for %q was not switched on: %v", slug, payer, err)
		return nil
	}
	card := defaultPaymentMethod(datastore.New(org.Namespaced(ctx)), payer) != nil
	state := armed
	v, _, err := writeRule(ctx, org, payer, func(c *autorecharge.AutoRecharge) bool {
		switch {
		case c.Enabled && c.Plan == "":
			state = kept
			return false
		case !card:
			state = needsCard
			c.Enabled, c.Plan, c.Waiting = false, slug, true
			return true
		}
		state = armed
		c.Enabled, c.ThresholdCents, c.AmountCents, c.Currency = true, in.ThresholdCents, in.AmountCents, ruleCurrencyOf(in.Currency)
		c.Plan, c.Waiting = slug, false
		return true
	})
	if err != nil {
		log.Error("auto-recharge: plan %q did not switch on the rule for %q: %v", slug, payer, err)
		return nil
	}
	switch state {
	case armed:
		log.Info("auto-recharge: plan %q switched on the rule for %q: reload %d %s below %d, at most %d a day (0 = unbounded)",
			slug, payer, v.AmountCents, v.Currency, v.ThresholdCents, v.PerDay)
	case needsCard:
		log.Info("auto-recharge: plan %q waits for a card on file to switch on the rule for %q", slug, payer)
	}
	return &Arming{State: state}
}

// armOnCard switches on, once a card is saved for the payer subject names
// (payerOf; a subject outside the org arms nothing), the reload a purchase left
// waiting for one: the payer's rule is waiting (its sale answered needs_card) for
// the plan the payer is served from now, on a period paid rather than a trial. No
// other rule is touched — not one the payer set, and not a plan the buyer bought
// before its row stated a reload. It never fails the save.
func armOnCard(ctx context.Context, org *organization.Organization, subject string) {
	payer, err := payerOf(org, subject)
	if err != nil {
		return
	}
	db := datastore.New(org.Namespaced(ctx))
	cfg := loadAutoRecharge(db, payer)
	if cfg == nil || !cfg.Waiting || cfg.Plan == "" {
		return
	}
	s := servedSubscription(db, payer, org.TestMode())
	if s == nil || s.Status != subscription.Active || servedAs(subscriptionSlug(s)) != cfg.Plan {
		return
	}
	armForPlan(ctx, org, payer, cfg.Plan, s.Plan.Currency)
}

// disarm switches off the rule the plan slug switched on for payer — or left
// waiting for a card — once payer no longer holds that plan: it was cancelled, now
// or at its period's end, or left for a plan with no reload. The rule stays on
// file switched off, at the figures the plan set; a rule the payer set themselves
// names no plan and is never touched. It never fails the act that called it.
func disarm(ctx context.Context, org *organization.Organization, payer, slug string) {
	if slug == "" {
		return
	}
	db := datastore.New(org.Namespaced(ctx))
	_, off, err := changeRule(db, payer, func(c *autorecharge.AutoRecharge) bool {
		if c.Plan != slug {
			return false
		}
		c.Enabled, c.Plan, c.Waiting = false, "", false
		return true
	})
	switch {
	case err != nil:
		log.Error("auto-recharge: the rule plan %q switched on for %q was not switched off: %v", slug, payer, err)
	case off:
		log.Info("auto-recharge: %q no longer holds plan %q; the rule it switched on is off", payer, slug)
	}
}

// GetAutoRecharge returns the caller's payer's auto-recharge rule (or the disabled
// default): the account the caller's credential pays from (userBillingKey).
//
//	GET /v1/billing/recharge
func GetAutoRecharge(c *zip.Ctx) error {
	org, ok := middleware.GetOrganizationOK(c)
	payer := userBillingKey(c)
	if !ok || org == nil || payer == "" {
		return jsonhttp.Fail(c, 401, "sign in to read auto-reload", nil)
	}
	v, err := ReadAutoRecharge(c.Context(), org, payer)
	if err != nil {
		log.Error("Failed to read auto-recharge config for %q: %v", payer, err, c)
		return jsonhttp.Fail(c, 500, "failed to read auto-recharge config", err)
	}
	b, _ := stdjson.Marshal(autoRechargeResponse(v))
	c.SetHeader("Content-Type", "application/json")
	return c.Bytes(200, b)
}

// SetAutoRecharge upserts the caller's payer's auto-recharge rule.
//
//	PUT /v1/billing/recharge
//
// Enabling requires a default payment method on file for the payer (the card
// that will be charged off-session when the balance runs low).
//
// The rule charges the payer's card, so it is set by whoever may change what that
// wallet pays with (mayChangeWallet): the org's admins, a platform SuperAdmin or a
// service for the org's own account, and a person for a wallet of their own. A
// member billed from the org's account is told to ask an admin.
func SetAutoRecharge(c *zip.Ctx) error {
	org, ok := middleware.GetOrganizationOK(c)
	payer := userBillingKey(c)
	if !ok || org == nil || payer == "" {
		return jsonhttp.Fail(c, 401, "sign in to change auto-reload", nil)
	}
	if !mayChangeWallet(c) {
		return jsonhttp.Fail(c, 403, askAnAdmin(c, "change its auto-reload"), nil)
	}

	var in AutoRechargeEdit
	if err := c.Bind(&in); err != nil {
		return jsonhttp.Fail(c, 400, "invalid request body", err)
	}

	v, err := WriteAutoRecharge(c.Context(), org, payer, in)
	if err != nil {
		if IsAutoRechargeRefused(err) {
			return jsonhttp.Fail(c, 400, err.Error(), nil)
		}
		log.Error("Failed to save auto-recharge config for %q: %v", payer, err, c)
		return jsonhttp.Fail(c, 500, "failed to save auto-recharge config", err)
	}

	return c.JSON(200, autoRechargeResponse(v))
}

// RechargeResult is one payer's outcome in a sweep — charged, skipped, or failed
// with the reason. UserId is the payer: the org's own account (the org slug) or a
// person's "<org>/<name>". The amount, balance and transaction are absent on
// anything but a charge, which is exactly what omitempty says here: a skipped
// payer has no amount, rather than an amount of zero.
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
// many payers were charged, and what happened to each one that was not simply
// skipped. Orgs is the population, not the row count — that difference is how a
// reader knows a quiet run swept anything at all.
type RechargeRun struct {
	Orgs    int              `json:"orgs"`
	Charged int              `json:"charged"`
	Results []RechargeResult `json:"results"`
}

// RunAutoRecharge sweeps every payer's rule in every organization and, for each
// rule switched on whose payer's available balance has fallen below its
// threshold, charges that payer's default card and credits that payer's balance.
//
// It takes values because the caller is a SCHEDULE, not a person: there is no
// request behind an off-session charge, and the one thing that must never be
// read from one is the retry key. A single run-all request's header would be
// shared by every payer this loop charges, which is why each reload derives its
// own key from its own trigger instead (reloadKey).
//
// A failure to charge one payer is that payer's result, not the sweep's: the loop
// continues, so one bad card cannot leave every later customer unfunded. Only
// the population read failing ends the run.
func RunAutoRecharge(ctx context.Context, ev *events.Client) (*RechargeRun, error) {
	rootDb := datastore.New(ctx)
	orgs := make([]*organization.Organization, 0)
	if _, err := organization.Query(rootDb).GetAll(&orgs); err != nil {
		return nil, err
	}

	run := &RechargeRun{Orgs: len(orgs), Results: make([]RechargeResult, 0)}
	for _, org := range orgs {
		db := datastore.New(org.Namespaced(ctx))
		for _, payer := range payers(db, org) {
			r, ok := reloadPayer(ctx, ev, org, db, payer)
			if !ok {
				continue
			}
			if r.Charged {
				run.Charged++
			}
			run.Results = append(run.Results, r)
		}
	}
	return run, nil
}

// payers are the payers in db with a rule on file, each once, in order: the org's
// own account and the accounts within it (payerOf). A rule keyed by any other
// subject is never swept, and how many there are is logged so they can be found.
func payers(db *datastore.Datastore, org *organization.Organization) []string {
	rootKey := db.NewKey("synckey", "", 1, nil)
	rows := make([]*autorecharge.AutoRecharge, 0)
	if _, err := autorecharge.Query(db).Ancestor(rootKey).GetAll(&rows); err != nil {
		log.Error("auto-recharge: could not list the rules of %s: %v", org.Name, err)
		return nil
	}
	out := make([]string, 0, len(rows))
	foreign := 0
	for _, r := range rows {
		payer, err := payerOf(org, r.UserId)
		if err != nil || r.UserId == "" || payer != r.UserId {
			foreign++
			continue
		}
		if !slices.Contains(out, payer) {
			out = append(out, payer)
		}
	}
	if foreign > 0 {
		log.Warn("auto-recharge: %d rule rows in %s are keyed outside its accounts and are not swept", foreign, org.Name)
	}
	slices.Sort(out)
	return out
}

// reloadKey is the idempotency key of the reload answering cfg's open trigger: the
// payer, the rule, the trigger (Seq, the reloads the rule has completed) and the
// processor's refusals of it (Tries).
//
// It is the duplicate guard, and nothing else. Every sweep that asks one trigger —
// two that overlap, or one retried after its answer was lost — sends the same key
// with the same amount on the same card (Amount and Card, fixed when the trigger is
// reserved), so the charge guard and the processor replay the first charge instead
// of making a second, whatever the rule or the default card has become since. Once
// a reload is credited Seq moves on, and a balance drained below the threshold again
// is the next trigger, with a key of its own however soon after the last reload it
// falls. A charge the processor refused (processor.Answer) moves Tries on, so the
// card is asked again under a new key rather than answered from the refusal; a
// charge with no answer — a lost one, or a key the processor says an earlier request
// holds — keeps its key, amount and card, because it may have moved money.
func reloadKey(cfg *autorecharge.AutoRecharge, payer string) string {
	return "reload:" + payer + ":rule:" + cfg.Id() + ":n:" + strconv.FormatInt(cfg.Seq, 10) + ":try:" + strconv.Itoa(cfg.Tries)
}

// owed reports whether a rule's payer, holding available, is below its threshold
// on a trigger the rule has not yet reloaded: never reloaded (Seq 0), or below the
// balance its last reload left (After), so money was spent since. A balance still
// below the threshold right after a reload is that reload's trigger, not a new one.
func owed(c *autorecharge.AutoRecharge, available int64) bool {
	return c.Enabled && c.AmountCents > 0 && available < c.ThresholdCents && (c.Seq == 0 || available < c.After)
}

// reloadPayer runs one payer's rule, in the order that settles money first: a
// charge that settled and is not yet credited is credited (creditReload); an open
// trigger whose charge has no answer is asked again exactly (askReload); and only
// then is a new trigger looked for. ok is false when there is nothing to report.
//
// The balance is the payer's prepaid money in the ledger the spend gate reads
// (prepaid.balance): the host's when one is injected, commerce's own otherwise.
// A read that fails changes nothing, and the next sweep reads again.
func reloadPayer(ctx context.Context, ev *events.Client, org *organization.Organization, db *datastore.Datastore, payer string) (RechargeResult, bool) {
	cfg := loadAutoRecharge(db, payer)
	switch {
	case cfg == nil:
		return RechargeResult{}, false
	case cfg.Ref != "":
		return creditReload(ctx, org, db, payer, cfg)
	case cfg.Amount > 0 && !cfg.Enabled:
		// Switched off with a charge unanswered: it is never asked again, because a
		// charge that never reached the processor would be made now, after the payer
		// said stop. It is the operator's to reconcile under its key.
		log.Error("RECONCILE: auto-reload for %q was switched off with %d unanswered under %s", payer, cfg.Amount, reloadKey(cfg, payer))
		return RechargeResult{OrgName: org.Name, UserId: payer, Error: "switched off with a reload unanswered"}, true
	case cfg.Amount > 0:
		return askReload(ctx, ev, org, db, payer, cfg, false)
	case !cfg.Enabled || cfg.AmountCents <= 0:
		return RechargeResult{}, false
	}

	available, err := prepaidFor(ctx, org).balance(ctx, payer, ruleCurrency(cfg))
	if err != nil {
		return RechargeResult{OrgName: org.Name, UserId: payer, Error: "the balance could not be read: " + err.Error()}, true
	}
	seen := cfg.Seq
	if !owed(cfg, available) {
		return RechargeResult{}, false
	}
	pm := defaultPaymentMethod(db, payer)
	if pm == nil {
		return RechargeResult{OrgName: org.Name, UserId: payer, Error: "no default payment method"}, true
	}

	// The trigger is RESERVED before any card is touched, under compare-and-swap
	// against the rule as it stands now: a rule switched off or changed since it
	// was read is not charged; a trigger another sweep has opened (Amount) or
	// closed since this one read the balance (Seq moved) is not opened again; and
	// the day's count holds however many sweeps overlap. The reservation fixes the
	// trigger's charge — the rule's amount, the payer's default card, the balance it
	// was read at. A payer that has reloaded as many times today as its plan allows
	// (reloadsFor) waits for tomorrow, and says so in its row; a plan that bounds
	// nothing is reloaded whenever it is due, the day's count kept all the same. A
	// card the processor refuses is asked at most ReloadsPerDay times a day, on
	// every plan.
	today := time.Now().UTC().Format(time.DateOnly)
	bound := reloadsFor(ctx, db, org, payer)
	full := ""
	cfg, reserved, rerr := changeRule(db, payer, func(c *autorecharge.AutoRecharge) bool {
		if c.Seq != seen || c.Amount > 0 || c.Ref != "" || !owed(c, available) {
			return false
		}
		if c.Day != today {
			c.Day, c.Reloads, c.Declines = today, 0, 0
		}
		switch {
		case c.Declines >= ReloadsPerDay:
			full = fmt.Sprintf("auto-reload's card declined %d times today", c.Declines)
			return false
		case bound > 0 && c.Reloads >= bound:
			full = fmt.Sprintf("auto-reload already charged %d times today", bound)
			return false
		}
		c.Reloads++
		c.Amount, c.Card, c.Base = c.AmountCents, pm.Id(), available
		return true
	})
	switch {
	case rerr != nil:
		return RechargeResult{OrgName: org.Name, UserId: payer, Error: rerr.Error()}, true
	case full != "":
		return RechargeResult{OrgName: org.Name, UserId: payer, Error: full}, true
	case !reserved:
		return RechargeResult{}, false
	}
	return askReload(ctx, ev, org, db, payer, cfg, true)
}

// askReload asks the processor for cfg's open trigger: Amount on Card under the
// trigger's key. fresh says this sweep reserved the trigger, and alone may give the
// reservation back. A settled charge is indexed to the payer (models/paymentorg),
// so a refund or dispute of it finds the wallet it credited, recorded on the rule
// as awaiting its credit (Ref), and credited (creditReload).
func askReload(ctx context.Context, ev *events.Client, org *organization.Organization, db *datastore.Datastore, payer string, cfg *autorecharge.AutoRecharge, fresh bool) (RechargeResult, bool) {
	seq, tries := cfg.Seq, cfg.Tries
	today := time.Now().UTC().Format(time.DateOnly)
	cur := ruleCurrency(cfg)
	pm := cardOf(db, payer, cfg.Card)
	if pm == nil {
		if fresh {
			release(db, payer, today, seq, tries)
		}
		log.Error("RECONCILE: auto-reload for %q is open on card %s, which is no longer on file", payer, cfg.Card)
		return RechargeResult{OrgName: org.Name, UserId: payer, Error: "the card the reload was asked on is no longer on file"}, true
	}

	desc := fmt.Sprintf("Auto-recharge %d %s for %s", cfg.Amount, cur, payer)
	r, err := chargeAndCredit(ctx, ev, org, db, pm, cfg.Amount, cur, payer, reloadKey(cfg, payer), desc)
	if err != nil {
		// Refused before the processor was asked: the reservation is given back by
		// the sweep that made it. Refused by the processor (processor.Answer): it
		// moved nothing, and the trigger's try moves on. Anything else may have moved
		// money: the trigger stays open, and the next sweep asks it again exactly.
		switch {
		case unasked(err):
			if fresh {
				release(db, payer, today, seq, tries)
			}
		case answered(err):
			refused(db, payer, today, seq, tries)
		}
		log.Error("Auto-recharge charge failed for %q: %v", payer, err)
		return RechargeResult{OrgName: org.Name, UserId: payer, Error: err.Error()}, true
	}

	ref := cmp.Or(r.ProcessorRef, r.TransactionId)
	if err := paymentorg.Put(ref, org.Name, payer, "", org.TestMode()); err != nil {
		log.Warn("auto-recharge: payment %s: the index to %q was not written, so its refund or dispute will not find the wallet it credited: %v", ref, payer, err)
	}
	cfg, _, err = changeRule(db, payer, func(c *autorecharge.AutoRecharge) bool {
		if c.Seq != seq || c.Tries != tries || c.Ref != "" {
			return false
		}
		c.Ref, c.Amount, c.Txn = ref, r.AmountCents, r.TransactionId
		return true
	})
	if err != nil || cfg == nil || cfg.Ref != ref {
		// Not recorded here: another sweep recorded or closed the same charge, or the
		// store failed and the next sweep asks the trigger again, replaying this one.
		if err != nil {
			log.Error("auto-recharge: the reload %s for %q was charged and not recorded: %v", ref, payer, err)
		}
		return RechargeResult{}, false
	}
	return creditReload(ctx, org, db, payer, cfg)
}

// creditReload credits the settled charge cfg awaits (Ref, Amount) to the ledger
// the spend gate reads (fund) and closes its trigger: Seq moves on, Tries starts
// again, and After is the balance the trigger was read at plus the amount reloaded.
// A credit that fails leaves the charge awaiting it, and every sweep credits it
// again under the processor's reference — which the ledger takes once — whatever
// the balance has done meanwhile.
func creditReload(ctx context.Context, org *organization.Organization, db *datastore.Datastore, payer string, cfg *autorecharge.AutoRecharge) (RechargeResult, bool) {
	cur := ruleCurrency(cfg)
	ref, amount, base, txn := cfg.Ref, cfg.Amount, cfg.Base, cfg.Txn
	if err := fund(ctx, org, payer, cur, receipt{ProcessorRef: ref, AmountCents: amount}); err != nil {
		log.Error("RECONCILE: auto-reload %s for %q (%d %s) charged and the spendable balance was NOT credited: %v — credited again on the next sweep",
			ref, payer, amount, cur, err)
		return RechargeResult{OrgName: org.Name, UserId: payer, Error: "the reload was charged and not yet credited: " + err.Error()}, true
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, closed, err := changeRule(db, payer, func(c *autorecharge.AutoRecharge) bool {
		if c.Ref != ref {
			return false
		}
		c.Seq, c.Tries, c.After = c.Seq+1, 0, base+amount
		c.Amount, c.Card, c.Base, c.Ref = 0, "", 0, ""
		c.LastRechargedAt = now
		return true
	})
	if err != nil {
		log.Error("auto-recharge: the reload %s for %q was credited and its trigger not closed: %v", ref, payer, err)
		return RechargeResult{OrgName: org.Name, UserId: payer, Error: err.Error()}, true
	}
	if !closed {
		return RechargeResult{}, false
	}
	// The balance the result reports is read now; the rule keeps none of it, so a
	// read that fails reports the balance the reload left and changes nothing.
	balance, err := prepaidFor(ctx, org).balance(ctx, payer, cur)
	if err != nil {
		balance = base + amount
	}
	return RechargeResult{
		OrgName:       org.Name,
		UserId:        payer,
		Charged:       true,
		AmountCents:   amount,
		Currency:      string(cur),
		Test:          org.TestMode(),
		BalanceCents:  balance,
		TransactionId: txn,
	}, true
}

// cardOf is the payment method id names when it is payer's own, nil otherwise.
func cardOf(db *datastore.Datastore, payer, id string) *paymentmethod.PaymentMethod {
	if id == "" {
		return nil
	}
	pm := paymentmethod.New(db)
	if err := pm.GetById(id); err != nil || pm.CustomerId != payer {
		return nil
	}
	return pm
}

// ruleCurrency is the currency a rule reloads in, usd when it names none.
func ruleCurrency(c *autorecharge.AutoRecharge) currency.Type {
	return currency.Type(ruleCurrencyOf(c.Currency))
}

// fund credits a reload's charge to the ledger the spend gate reads, when the host
// injected one, at the payer's own address and under the processor's reference
// for the charge: the key every path that can credit one payment uses, so the
// payment is credited once. Without a host ledger the charge's deposit is already
// in commerce's own ledger, which is the one read, and there is nothing to do.
func fund(ctx context.Context, org *organization.Organization, payer string, cur currency.Type, r receipt) error {
	led := creditledger.Get()
	if led == nil {
		return nil
	}
	if r.ProcessorRef == "" || r.AmountCents <= 0 {
		return fmt.Errorf("the charge's receipt names no processor reference and amount to credit under")
	}
	_, _, err := led.Credit(ctx, creditledger.CreditInput{
		Org:            ledgerOrg(org),
		Subject:        payer,
		Currency:       string(cur),
		Reason:         "auto-reload " + r.ProcessorRef,
		Tag:            "topup",
		IdempotencyKey: r.ProcessorRef,
		AmountCents:    r.AmountCents,
		Test:           org.TestMode(),
		Cash:           true,
	})
	return err
}

// release gives back a reservation that asked no processor: today's count and the
// open trigger, when the trigger is still the one reserved (seq, tries).
func release(db *datastore.Datastore, payer, today string, seq int64, tries int) {
	if _, _, err := changeRule(db, payer, func(c *autorecharge.AutoRecharge) bool {
		if c.Seq != seq || c.Tries != tries || c.Amount == 0 || c.Ref != "" {
			return false
		}
		if c.Day == today && c.Reloads > 0 {
			c.Reloads--
		}
		c.Amount, c.Card, c.Base = 0, "", 0
		return true
	}); err != nil {
		log.Error("auto-recharge: could not release a reload for %q: %v", payer, err)
	}
}

// refused records the processor's refusal of the open trigger (seq, tries): the
// trigger closes its charge, its next try is a key of its own, and today's
// refusals move toward ReloadsPerDay.
func refused(db *datastore.Datastore, payer, today string, seq int64, tries int) {
	if _, _, err := changeRule(db, payer, func(c *autorecharge.AutoRecharge) bool {
		if c.Seq != seq || c.Tries != tries || c.Amount == 0 || c.Ref != "" {
			return false
		}
		c.Tries++
		if c.Day == today {
			c.Declines++
		}
		c.Amount, c.Card, c.Base = 0, "", 0
		return true
	}); err != nil {
		log.Error("auto-recharge: could not count a refusal for %q: %v", payer, err)
	}
}

// stopReload switches payer's rule in org off when a payment it was credited from
// is refunded or disputed: the card is not charged off-session again until the
// payer switches the rule back on. It never fails the act that called it.
func stopReload(ctx context.Context, org, payer, why string) {
	db := datastore.New(nscontext.WithNamespace(ctx, org))
	_, off, err := changeRule(db, payer, func(c *autorecharge.AutoRecharge) bool {
		if !c.Enabled {
			return false
		}
		c.Enabled, c.Plan, c.Waiting = false, "", false
		return true
	})
	switch {
	case err != nil:
		log.Error("auto-recharge: the rule for %s/%q was not switched off after %s: %v", org, payer, why, err)
	case off:
		log.Info("auto-recharge: %s switched off the rule for %s/%q", why, org, payer)
	}
}

// RunAutoRechargeAllOrgs iterates every organization and, for those with
// auto-recharge enabled whose available balance is below the threshold, charges
// the default payment method and credits the balance. Intended to be invoked on
// a recurring schedule (CronJob) by the platform.
//
//	POST /v1/billing/recharge/run-all
func RunAutoRechargeAllOrgs(c *zip.Ctx) error {
	run, err := RunAutoRecharge(c.Context(), eventsOf(c))
	if err != nil {
		log.Error("Failed to list organizations for auto-recharge: %v", err, c)
		return jsonhttp.Fail(c, 500, "failed to list organizations", err)
	}
	return c.JSON(200, run)
}

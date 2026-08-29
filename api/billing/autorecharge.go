package billing

import (
	"context"
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
	if _, err := autorecharge.Query(db).Ancestor(rootKey).Filter("UserId=", userId).GetAll(&cfgs); err != nil {
		return nil
	}
	if len(cfgs) == 0 {
		return nil
	}
	// A query-loaded entity carries no datastore, so calling Update() on it
	// nil-derefs inside orm.Model.Key(). Both callers mutate and save what this
	// returns — SetAutoRecharge on an existing row, and the cron when it stamps
	// LastRechargedAt after a successful charge — so the binding belongs here,
	// once, rather than at each call site. Rebind, not Init: Init would re-apply
	// Defaults() over the values just loaded.
	cfgs[0].Rebind(db)
	return cfgs[0]
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

// The three refusals WriteAutoRecharge makes, each its own sentinel so a caller
// separates a mistake it can fix from a store that failed us WITHOUT reading the
// message. A caller matching on text is a caller that breaks when the text is
// reworded, and the endpoint below still hands these strings to the customer, so
// the wording is a customer-facing value rather than a private one.
//
// All three bite only when the rule is ARMED: a disabled rule charges nobody, so
// a zero amount or a card not yet on file is a customer part-way through setting
// up, not an error.
var (
	ErrAmountNotPositive      = errors.New("amountCents must be positive")
	ErrThresholdNegative      = errors.New("thresholdCents must not be negative")
	ErrNoDefaultPaymentMethod = errors.New("a default payment method is required to enable auto-recharge")
)

// autoRechargeView renders a loaded row — or the absence of one — as the rule its
// readers see. An org with no row has the DISABLED rule in usd, which is the rule
// the sweep reads it as; that agreement is the point, since a reader shown one
// default and a sweep applying another is two rules again.
func autoRechargeView(cfg *autorecharge.AutoRecharge, subject string) AutoRecharge {
	if cfg == nil {
		return AutoRecharge{Subject: subject, Currency: "usd"}
	}
	return AutoRecharge{
		Subject:         cfg.UserId,
		Enabled:         cfg.Enabled,
		ThresholdCents:  cfg.ThresholdCents,
		AmountCents:     cfg.AmountCents,
		Currency:        cfg.Currency,
		LastRechargedAt: cfg.LastRechargedAt,
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
// A refusal of the caller's own values is one of the three sentinels above;
// anything else is the store failing us. Which status each becomes is the
// endpoint's business, not this one's.
func WriteAutoRecharge(ctx context.Context, org *organization.Organization, in AutoRechargeEdit) (AutoRecharge, error) {
	if org == nil {
		return AutoRecharge{}, errors.New("auto-recharge: no organization")
	}
	db := datastore.New(org.Namespaced(ctx))

	if in.Enabled {
		if in.AmountCents <= 0 {
			return AutoRecharge{}, ErrAmountNotPositive
		}
		if in.ThresholdCents < 0 {
			return AutoRecharge{}, ErrThresholdNegative
		}
		if defaultPaymentMethod(db, org.Name) == nil {
			return AutoRecharge{}, ErrNoDefaultPaymentMethod
		}
	}

	cur := strings.ToLower(strings.TrimSpace(in.Currency))
	if cur == "" {
		cur = "usd"
	}

	cfg := loadAutoRecharge(db, org.Name)
	if cfg == nil {
		cfg = autorecharge.New(db)
		cfg.UserId = org.Name
	}
	cfg.Enabled = in.Enabled
	cfg.ThresholdCents = in.ThresholdCents
	cfg.AmountCents = in.AmountCents
	cfg.Currency = cur

	var err error
	if cfg.Id() == "" {
		err = cfg.Create()
	} else {
		err = cfg.Update()
	}
	if err != nil {
		return AutoRecharge{}, err
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
	return c.JSON(200, autoRechargeResponse(v))
}

// SetAutoRecharge upserts the org's auto-recharge config.
//
//	PUT /v1/billing/recharge
//
// Enabling requires a default payment method on file (the card that will be
// charged off-session when the balance runs low).
func SetAutoRecharge(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)

	var in AutoRechargeEdit
	if err := c.Bind(&in); err != nil {
		return jsonhttp.Fail(c, 400, "invalid request body", err)
	}

	v, err := WriteAutoRecharge(c.Context(), org, in)
	if err != nil {
		switch {
		case errors.Is(err, ErrAmountNotPositive),
			errors.Is(err, ErrThresholdNegative),
			errors.Is(err, ErrNoDefaultPaymentMethod):
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

		pm := defaultPaymentMethod(db, org.Name)
		if pm == nil {
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
			log.Error("Auto-recharge charge failed for org %q: %v", org.Name, err)
			run.Results = append(run.Results, RechargeResult{
				OrgName: org.Name, UserId: org.Name, Charged: false, Error: err.Error(),
			})
			continue
		}

		cfg.LastRechargedAt = time.Now().UTC().Format(time.RFC3339)
		_ = cfg.Update()

		run.Charged++
		run.Results = append(run.Results, RechargeResult{
			OrgName:       org.Name,
			UserId:        org.Name,
			Charged:       true,
			AmountCents:   cfg.AmountCents,
			BalanceCents:  int64(balanceCents),
			TransactionId: txID,
		})
	}

	return run, nil
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

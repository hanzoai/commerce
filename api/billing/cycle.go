package billing

import (
	"context"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/billing/engine"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/thirdparty/kms"
	"github.com/hanzoai/commerce/util/json/http"
)

type cycleUserRequest struct {
	UserId string `json:"userId"`
}

// RunBillingCycle processes all subscriptions whose current period has ended
// for the request's organization. It generates invoices and attempts collection
// for each due subscription.
//
//	POST /v1/billing/cycle/run
func RunBillingCycle(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	hydratePaymentCreds(c, org)
	db := datastore.New(org.Namespaced(c.Context()))

	results := renewDue(c.Context(), org, "")

	// Top up each active subscriber's included monthly allotment for the
	// current period (idempotent per user+month).
	allotGranted, allotSkipped, _ := grantOrgAllotments(c, db, time.Now(), !org.TestMode())

	return c.JSON(200, map[string]any{
		"processed": len(results),
		"results":   results,
		"allotments": map[string]any{
			"granted": allotGranted,
			"skipped": allotSkipped,
		},
	})
}

// RunBillingCycleUser processes due subscriptions for a single user within the
// request's organization.
//
//	POST /v1/billing/cycle/run-user
func RunBillingCycleUser(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	hydratePaymentCreds(c, org)

	var req cycleUserRequest
	if err := c.Bind(&req); err != nil {
		return http.Fail(c, 400, "invalid request body", err)
	}

	if req.UserId == "" {
		return http.Fail(c, 400, "userId is required", nil)
	}

	results := renewDue(c.Context(), org, req.UserId)

	return c.JSON(200, map[string]any{
		"user":      req.UserId,
		"processed": len(results),
		"results":   results,
	})
}

// RunBillingCycleAllOrgs iterates every organization and processes due
// subscriptions across all of them, then grants each org's included allotments.
//
//	POST /v1/billing/cycle/run-all
func RunBillingCycleAllOrgs(c *zip.Ctx) error {
	run, err := RenewDue(c.Context(), kmsOf(c))
	if err != nil {
		log.Error("Failed to list organizations for billing cycle: %v", err, c)
		return http.Fail(c, 500, "failed to list organizations", err)
	}

	rootDb := datastore.New(c.Context())
	orgs := make([]*organization.Organization, 0)
	if _, err := organization.Query(rootDb).GetAll(&orgs); err == nil {
		now := time.Now()
		for _, org := range orgs {
			grantOrgAllotments(c, datastore.New(org.Namespaced(c.Context())), now, !org.TestMode())
		}
	}

	return c.JSON(200, run)
}

// Renewal is what renewing one due subscription did.
type Renewal struct {
	OrgName        string `json:"orgName"`
	UserId         string `json:"userId"`
	SubscriptionId string `json:"subscriptionId"`
	InvoiceId      string `json:"invoiceId,omitempty"`
	Success        bool   `json:"success"`
	Error          string `json:"error,omitempty"`
}

// RenewalRun is one sweep: how many organizations were considered, how many
// subscriptions were due, and what renewing each one did.
type RenewalRun struct {
	Orgs    int       `json:"orgs"`
	Due     int       `json:"due"`
	Renewed int       `json:"renewed"`
	Results []Renewal `json:"results"`
}

// RenewDue renews every due subscription in every organization: it invoices the
// period that ended and collects it through the one waterfall — prepaid money
// first, the card on file for the rest — moving the row on when it is paid and
// leaving it past due when it is not.
//
// It takes values because the caller is a SCHEDULE, not a person. A failure to
// collect one subscription is that subscription's result, not the sweep's: the
// loop continues. Only the population read failing ends the run.
func RenewDue(ctx context.Context, kmsClient *kms.CachedClient) (*RenewalRun, error) {
	orgs := make([]*organization.Organization, 0)
	if _, err := organization.Query(datastore.New(ctx)).GetAll(&orgs); err != nil {
		return nil, err
	}
	run := &RenewalRun{Orgs: len(orgs), Results: make([]Renewal, 0)}
	for _, org := range orgs {
		// Hydrate each org's payment creds so a due renewal can charge its
		// subscribers' vaulted cards through that org's own processor.
		if kmsClient != nil {
			if err := kms.Hydrate(kmsClient, org); err != nil {
				log.Error("KMS hydration failed for org %q during renewal: %v", org.Name, err)
			}
		}
		for _, r := range renewDue(ctx, org, "") {
			run.Due++
			if r.Success {
				run.Renewed++
			}
			run.Results = append(run.Results, r)
		}
	}
	return run, nil
}

// renewDue renews org's due subscriptions — one subject's when user is set. A row
// the sweep missed for whole periods is billed the period running now, never every
// period since (engine.RenewSubscription).
func renewDue(ctx context.Context, org *organization.Organization, user string) []Renewal {
	// Subscriptions are stored ancestor-less, so they are read the way every
	// other reader reads them. An Ancestor(synckey) filter matches none of them,
	// which left this sweep renewing nothing.
	subs, err := ListSubscriptions(ctx, org, user, "")
	if err != nil {
		log.Error("Failed to query subscriptions for renewal in org %q: %v", org.Name, err)
		return nil
	}
	db := datastore.New(org.Namespaced(ctx))

	now := time.Now()
	pre, charge := prepaidFor(ctx, org), chargeProviderForOrg(org)
	out := make([]Renewal, 0)
	for _, sub := range subs {
		// A row renews from the books of its own mode, and the org's prepaid money
		// is the org's mode: a sandbox row is never paid from live money, nor a
		// live row from sandbox money.
		if sub.Test != org.TestMode() || !engine.IsDue(sub, now) {
			continue
		}
		out = append(out, renewOne(ctx, db, org.Name, sub, pre, charge))
	}
	return out
}

// renewOne invoices and collects one subscription's due period, then persists
// the row as the collection left it.
func renewOne(ctx context.Context, db *datastore.Datastore, orgName string, sub *subscription.Subscription, pre engine.Prepaid, charge engine.ProviderCharger) Renewal {
	// A row read by query is not bound to the store, so the write goes through a
	// fresh read of the same id.
	row := subscription.New(db)
	if err := row.GetById(sub.Id()); err != nil {
		return Renewal{OrgName: orgName, UserId: sub.UserId, SubscriptionId: sub.Id(), Error: err.Error()}
	}
	r := Renewal{OrgName: orgName, UserId: row.UserId, SubscriptionId: row.Id()}
	inv, result, err := engine.RenewSubscription(ctx, db, row, pre, charge)
	if inv != nil {
		r.InvoiceId = inv.Id()
	}
	if err != nil {
		log.Error("Billing cycle: failed to renew subscription %s: %v", row.Id(), err)
		r.Error = err.Error()
		return r
	}
	if err := row.Update(); err != nil {
		log.Error("Billing cycle: failed to update subscription %s after renewal: %v", row.Id(), err)
		r.Error = err.Error()
		return r
	}
	r.Success, r.Error = result.Success, result.Error
	return r
}

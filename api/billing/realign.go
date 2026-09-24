package billing

import (
	"context"
	"fmt"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/billing/engine"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/util/json/http"
)

// RealignResult is one subscription the realignment moved back onto the period
// its paid invoice covers, or in a dry run would move.
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
}

// RealignReport is one realignment run: every subscription it moved, or in a
// dry run would move, out of the live ones it considered.
type RealignReport struct {
	DryRun     bool            `json:"dryRun"`
	Orgs       int             `json:"orgs"`
	Considered int             `json:"considered"`
	Realigned  int             `json:"realigned"`
	Results    []RealignResult `json:"results"`
	Errors     []string        `json:"errors,omitempty"`
}

// Realign corrects the dates of the subscriptions in orgs that sit a period
// ahead of the invoice that paid for them (engine.Realign): each moves back onto
// that invoice's period, and is renewed when that period ends. It is an
// operator's act, run on its own and never as part of reading or renewing a
// subscription. With dryRun every move is reported and nothing is written. Each
// org's run holds the org's cycle lock, so it never overlaps a cycle run of the
// same org in this process.
func Realign(ctx context.Context, orgs []*organization.Organization, dryRun bool) *RealignReport {
	report := &RealignReport{DryRun: dryRun, Orgs: len(orgs), Results: make([]RealignResult, 0)}
	for _, org := range orgs {
		realignOrg(ctx, org, dryRun, report)
	}
	return report
}

func realignOrg(ctx context.Context, org *organization.Organization, dryRun bool, report *RealignReport) {
	defer lockOrgCycle(org.Name)()
	db := datastore.New(org.Namespaced(ctx))
	subs, err := orgSubscriptions(db, "")
	if err != nil {
		log.Error("realign: list subscriptions for %q: %v", org.Name, err)
		report.Errors = append(report.Errors, fmt.Sprintf("%s: list subscriptions: %v", org.Name, err))
		return
	}
	for _, s := range subs {
		if !engine.Live(s) {
			continue
		}
		report.Considered++
		r, err := engine.Realign(db, s, dryRun)
		if err != nil {
			log.Error("realign: subscription %s in %q: %v", s.Id(), org.Name, err)
			report.Errors = append(report.Errors, fmt.Sprintf("%s: subscription %s: %v", org.Name, s.Id(), err))
			continue
		}
		if r == nil {
			continue
		}
		report.Realigned++
		report.Results = append(report.Results, RealignResult{
			Org: org.Name, SubscriptionId: s.Id(), UserId: s.UserId, Plan: planOf(s), Interval: string(s.Plan.Interval),
			InvoiceId: r.InvoiceId, FromStart: r.FromStart, FromEnd: r.FromEnd, ToStart: r.ToStart, ToEnd: r.ToEnd,
		})
	}
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

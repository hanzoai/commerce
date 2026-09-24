package billing

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/hanzoai/commerce/billing/tier"
	"github.com/hanzoai/commerce/models/organization"
	txutil "github.com/hanzoai/commerce/models/transaction/util"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/test/ae"
)

// H1: the ledger's test bucket MUST follow the org's own transacting mode
// (org.TestMode, now a per-tenant fact rather than a deployment-wide env var).
// These drive the REAL top-up handler and prove credit-bucket == read-bucket in
// both directions, and that the OPPOSITE bucket stays empty — closing the
// sandbox-charge -> live-credit (free money) and live-charge -> test-credit
// (mislabeled revenue) holes.

func bucketBalance(t *testing.T, ctx context.Context, user string, test bool) currency.Cents {
	t.Helper()
	datas, err := txutil.GetTransactionsByCurrency(ctx, user, "iam-user", currency.USD, test)
	if err != nil {
		t.Fatalf("GetTransactionsByCurrency(test=%v): %v", test, err)
	}
	if d, ok := datas.Data[currency.USD]; ok && d != nil {
		return d.Balance
	}
	return 0
}

// Matrix A — the FREE-MONEY case: an org in TEST mode. Its charge must credit
// the TEST bucket; the LIVE (spendable) bucket must stay empty. Driven through
// the REAL top-up handler, so this pins the shipped path and not a copy of it.
func TestH1_TestModeOrg_CreditsTestBucketNotLive(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	org := &organization.Organization{}
	org.Name = "h1-testmode"
	org.Live = false
	if !org.TestMode() {
		t.Fatal("precondition: a non-live org must be in test mode")
	}
	m := squareMock("cust_a", "ccof_a", "sqpay_a")
	withFakeSquare(t, m)

	if r := invokeTopupToken(org, ctx, `{"sourceId":"cnon:a","amountCents":500}`, nil); r.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(r.Body)
		t.Fatalf("status=%d body=%s, want 200", r.StatusCode, string(b))
	}

	nsctx := org.Namespaced(ctx)
	if got := bucketBalance(t, nsctx, "h1-testmode", true); got != 500 {
		t.Fatalf("TEST bucket = %d, want 500 (a test-mode charge credits the test bucket)", got)
	}
	if got := bucketBalance(t, nsctx, "h1-testmode", false); got != 0 {
		t.Fatalf("LIVE bucket = %d, want 0 — FREE-MONEY hole: a sandbox charge credited the spendable live balance", got)
	}
}

// Matrix B — a LIVE org: the charge is real, so it must book the LIVE bucket
// (real revenue), never the test bucket.
func TestH1_LiveOrg_CreditsLiveBucketNotTest(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	org := moneyOrg("h1-live")
	if org.TestMode() {
		t.Fatal("precondition: a live org must not be in test mode")
	}
	m := squareMock("cust_b", "ccof_b", "sqpay_b")
	withFakeSquare(t, m)

	if r := invokeTopupToken(org, ctx, `{"sourceId":"cnon:b","amountCents":700}`, nil); r.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(r.Body)
		t.Fatalf("status=%d body=%s, want 200", r.StatusCode, string(b))
	}

	nsctx := org.Namespaced(ctx)
	if got := bucketBalance(t, nsctx, "h1-live", false); got != 700 {
		t.Fatalf("LIVE bucket = %d, want 700 (a live charge books real revenue)", got)
	}
	if got := bucketBalance(t, nsctx, "h1-live", true); got != 0 {
		t.Fatalf("TEST bucket = %d, want 0 — production revenue mislabeled as test (under-pays OSS payout)", got)
	}
}

// Matrix C — a SUBSCRIPTION bought in test mode. Square's sandbox settles its
// public test card, so a sandbox sale is money that never moved. It must open a
// test subscription: a test-mode reader sees the plan, a live reader sees
// nothing — no paid tier, no included allotment.
func TestH1_TestModeSale_OpensNoLiveEntitlement(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	org := &organization.Organization{}
	org.Name = "h1-testsale"
	org.Live = false
	withFakeSquare(t, squareMock("cust_c", "ccof_c", "sqpay_c"))

	if r := invokeSubscribeCard(org, ctx, `{"sourceId":"cnon:card-nonce-ok","planId":"dev"}`, nil); r.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(r.Body)
		t.Fatalf("status=%d body=%s, want 201", r.StatusCode, string(b))
	}

	live := org.In(organization.Live)
	if got, err := TierOf(ctx, &live, "h1-testsale"); err != nil || got != tier.Free {
		t.Fatalf("live tier = %q (err %v), want free — a sandbox sale opened a paid tier", got, err)
	}
	if got, err := TierOf(ctx, org, "h1-testsale"); err != nil || got != tier.Pro {
		t.Fatalf("test-mode tier = %q (err %v), want pro — the test purchase must still work in test mode", got, err)
	}
	roll, err := ReadRollup(ctx, &live, "h1-testsale", "", time.Now())
	if err != nil {
		t.Fatalf("ReadRollup(live): %v", err)
	}
	if roll.Plan != "" || roll.Included.MonthlyCents != 0 {
		t.Fatalf("live rollup plan=%q included=%d, want none — a sandbox sale anchored a live allotment", roll.Plan, roll.Included.MonthlyCents)
	}
}

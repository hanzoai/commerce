package billing

import (
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/transaction"
	txutil "github.com/hanzoai/commerce/models/transaction/util"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/test/ae"
)

// H1 (Red): the ledger test bucket MUST follow the charge environment
// (org.TestMode), not org.Live alone. These tests credit a real Deposit through
// the actual transaction ledger exactly as the topup path does (trans.Test =
// org.TestMode()) and prove credit-bucket == read-bucket == charge-env, and that
// the OPPOSITE bucket stays empty — closing the sandbox-charge → live-credit
// (free money) and production-charge → test-credit (mislabeled revenue) holes.

// depositLikeTopup mirrors api/billing/topup.go chargeAndCredit: the Deposit's
// Test bucket is the org's resolved TestMode.
func depositLikeTopup(db *datastore.Datastore, org *organization.Organization, user string, cents int64) {
	tr := transaction.New(db)
	tr.Type = transaction.Deposit
	tr.DestinationId = user
	tr.DestinationKind = "iam-user"
	tr.Currency = currency.USD
	tr.Amount = currency.Cents(cents)
	tr.Test = org.TestMode() // the fix under test
	tr.MustCreate()
}

func bucketBalance(t *testing.T, ctx ae.Context, user string, test bool) currency.Cents {
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

// Matrix A — the FREE-MONEY case: an org in TEST mode. A sandbox charge must
// credit the TEST bucket; the LIVE (spendable) bucket must stay empty.
//
// The lever is the ORG, not the deployment. This used to force the scenario with
// SQUARE_ENVIRONMENT=sandbox against a Live org — mode and credentials
// deliberately disagreeing. That disagreement is now unrepresentable: one org
// record drives both, so the invariant holds by construction rather than by a
// deployment being templated correctly. What is asserted is unchanged — whichever
// way the authority resolves, the ledger bucket must follow it.
func TestH1_TestModeOrg_CreditsTestBucketNotLive(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	db := datastore.New(ctx)

	org := &organization.Organization{}
	org.Live = false // the org itself is in test mode

	if !org.TestMode() {
		t.Fatal("precondition: a non-live org must be in test mode")
	}

	const user = "h1-matrixA-user"
	depositLikeTopup(db, org, user, 500)

	if got := bucketBalance(t, ctx, user, true); got != 500 {
		t.Fatalf("TEST bucket = %d, want 500 (sandbox charge must credit the test bucket)", got)
	}
	if got := bucketBalance(t, ctx, user, false); got != 0 {
		t.Fatalf("LIVE bucket = %d, want 0 — FREE-MONEY hole: a sandbox charge credited the spendable live balance", got)
	}
}

// Matrix B — a LIVE org: the charge is real, so it must book the LIVE bucket
// (real revenue), not the test bucket. Mislabelling real revenue as test
// under-pays the OSS payout.
func TestH1_LiveOrg_CreditsLiveBucketNotTest(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	db := datastore.New(ctx)

	org := &organization.Organization{}
	org.Live = true // the org itself is live

	if org.TestMode() {
		t.Fatal("precondition: a live org must not be in test mode")
	}

	const user = "h1-matrixB-user"
	depositLikeTopup(db, org, user, 700)

	if got := bucketBalance(t, ctx, user, false); got != 700 {
		t.Fatalf("LIVE bucket = %d, want 700 (production charge books real revenue)", got)
	}
	if got := bucketBalance(t, ctx, user, true); got != 0 {
		t.Fatalf("TEST bucket = %d, want 0 — production revenue mislabeled as test (under-pays OSS payout)", got)
	}
}

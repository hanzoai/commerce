package billing

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/refund"
	"github.com/hanzoai/commerce/util/test/ae"
)

// Commerce never makes a refund. One is recorded only from the processor's signed
// webhook, under the processor's own refund id, through one locked writer.

// A refund the processor reports more than once — created, then updated, each
// COMPLETED — is one refund.
func TestWebhook_ARefundReportedTwiceCountsOnce(t *testing.T) {
	const secret = "whsec_rf_twice"
	registerSquare(t, secret)
	ctx := ae.NewContext()
	defer ctx.Close()
	org := webhookOrg(t, ctx, "rf-twice", true)
	withFakeSquare(t, squareMock("cust_tw", "ccof_tw", "sqpay_tw"))
	subscribed(t, ctx, org, "rf-twice", "team", 2, nil)

	deliver(t, ctx, secret,
		refundEvent("evt_rf_tw_0", "refund.created", "sqrf_tw", "sqpay_tw", "COMPLETED", 2500),
		refundEvent("evt_rf_tw_1", "refund.updated", "sqrf_tw", "sqpay_tw", "COMPLETED", 2500))
	paidAs(t, viewOf(t, ctx, org, "rf-twice"), "card", 2, 2500)
}

// Refunds of one payment landing at once are all kept: each is written under the
// one lock, onto the invoice as it stands then.
func TestWebhook_RefundsLandingTogetherAreAllKept(t *testing.T) {
	const secret = "whsec_rf_together"
	registerSquare(t, secret)
	ctx := ae.NewContext()
	defer ctx.Close()
	org := webhookOrg(t, ctx, "rf-together", true)
	withFakeSquare(t, squareMock("cust_tg", "ccof_tg", "sqpay_tg"))
	subscribed(t, ctx, org, "rf-together", "team", 2, nil)

	const n = 8
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			b := refundEvent(fmt.Sprintf("evt_rf_tg_%d", i), "refund.updated", fmt.Sprintf("sqrf_tg_%d", i), "sqpay_tg", "COMPLETED", 100)
			if r := deliverWebhook(ctx, "", secret, b, ""); r.StatusCode != http.StatusOK {
				t.Errorf("delivery %d: %d", i, r.StatusCode)
			}
		}(i)
	}
	wg.Wait()
	paidAs(t, viewOf(t, ctx, org, "rf-together"), "card", 2, 5000-n*100)
}

// POST /v1/billing/refunds and /v1/billing/refund are not routed, even for the
// platform, and a request to either writes nothing: no refund row, and the invoice
// holds all it was paid.
func TestRefundsAPI_POSTIsNotRoutedAndWritesNothing(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("rf-gone")
	withFakeSquare(t, squareMock("cust_gone", "ccof_gone", "sqpay_gone"))
	sub := subscribed(t, ctx, org, "rf-gone", "team", 2, nil)
	app := engineWithSeed(func(c *zip.Ctx) {
		c.Locals("organization", org)
		c.SetContext(ctx)
		platformApp(c)
	})

	for path, body := range map[string]string{
		"/v1/billing/refunds": fmt.Sprintf(`{"invoiceId":%q,"amount":1000}`, sub.CurrentInvoiceId),
		"/v1/billing/refund":  `{"user":"rf-gone","amount":1000,"originalTransactionId":"x"}`,
	} {
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("POST %s: %v", path, err)
		}
		if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusMethodNotAllowed {
			t.Fatalf("POST %s: %d %s; want 404 or 405", path, resp.StatusCode, bodyOf(resp))
		}
	}
	rows := make([]*refund.Refund, 0)
	if _, err := refund.Query(datastore.New(org.Namespaced(ctx))).GetAll(&rows); err != nil || len(rows) != 0 {
		t.Fatalf("refund rows = %d (%v); want none", len(rows), err)
	}
	paidAs(t, viewOf(t, ctx, org, "rf-gone"), "card", 2, 5000)
}

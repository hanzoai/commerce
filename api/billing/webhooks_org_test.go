// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package billing

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/checkout"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/billingevent"
	"github.com/hanzoai/commerce/models/paymentorg"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/payment/processor"
	squareprovider "github.com/hanzoai/commerce/payment/providers/square"
	"github.com/hanzoai/commerce/util/test/ae"
)

// post delivers body to path, signed for the registered verifier, with any extra
// request headers a replayer would add.
func post(ctx ae.Context, path, secret string, body []byte, headers map[string]string) (int, string) {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(string(body)))
	req.Header.Set(squareSigHeader, squareSign(secret, testWebhookURL, body))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	r := driveSeeded(func(c *zip.Ctx) { c.SetContext(ctx) }, "/v1/commerce/webhooks/:provider", req, HandleProviderWebhook)
	raw, _ := io.ReadAll(r.Body)
	return r.StatusCode, string(raw)
}

// recorded reports whether orgName's billing events hold eventID.
func recorded(t *testing.T, ctx ae.Context, orgName, eventID string) bool {
	t.Helper()
	org := webhookOrg(t, ctx, orgName, true)
	found, err := billingevent.New(datastore.New(org.Namespaced(ctx))).Query().Filter("ObjectId=", eventID).Get()
	if err != nil {
		t.Fatal(err)
	}
	return found
}

// The org a delivery is booked to is the verifying account's, else the
// deployment's own brand — never the request's.
func TestBookedOrgIsTheVerifyingAccounts(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	if got := bookedOrg(ctx, &processor.WebhookEvent{Org: "acme"}); got == nil || got.Name != "acme" {
		t.Fatalf("an account owned by acme booked to %+v", got)
	}
	if got := bookedOrg(ctx, &processor.WebhookEvent{}); got == nil || got.Name != checkout.DefaultBrandSlug() {
		t.Fatalf("the deployment's account booked to %+v, want %q", got, checkout.DefaultBrandSlug())
	}
}

// A replayer's identity headers do not move a delivery into their org.
func TestWebhookIgnoresTheRequestsOrg(t *testing.T) {
	const secret = "whsec_noheader"
	registerSquare(t, secret)
	ctx := ae.NewContext()
	defer ctx.Close()
	body := squareEvent("evt_noheader", "sub_noheader", time.Now())
	if st, raw := post(ctx, "/v1/commerce/webhooks/square", secret, body, map[string]string{"X-Org-Id": "victim", "X-User-Id": "mallory"}); st != http.StatusOK {
		t.Fatalf("status %d %s", st, raw)
	}
	if recorded(t, ctx, "victim", "evt_noheader") {
		t.Fatal("the delivery was booked to the org a header named")
	}
	if !recorded(t, ctx, checkout.DefaultBrandSlug(), "evt_noheader") {
		t.Fatal("the deployment's org holds no record of the delivery")
	}
}

// The path names the one processor that verifies; a name no processor holds is
// refused, so a genuine body cannot be re-presented under fresh path names.
func TestWebhookPathNamesTheProcessor(t *testing.T) {
	const secret = "whsec_path"
	registerSquare(t, secret)
	ctx := ae.NewContext()
	defer ctx.Close()
	body := squareEvent("evt_path", "sub_path", time.Now())
	for _, p := range []string{"squarex", "anything-at-all", "stripe"} {
		if st, raw := post(ctx, "/v1/commerce/webhooks/"+p, secret, body, nil); st != http.StatusUnauthorized && st != http.StatusBadRequest {
			t.Fatalf("a Square delivery at /%s answered %d %s, want a refusal", p, st, raw)
		}
	}
}

// One event is applied once for the whole deployment: a second delivery is a
// duplicate whatever path or account it arrives under.
func TestWebhookEventIsAppliedOnceEverywhere(t *testing.T) {
	const secret = "whsec_once"
	registerSquare(t, secret)
	ctx := ae.NewContext()
	defer ctx.Close()
	body := squareEvent("evt_once", "sub_once", time.Now().Add(-48*time.Hour))
	if st, raw := post(ctx, "/v1/commerce/webhooks/square", secret, body, nil); st != http.StatusOK || strings.Contains(raw, "duplicate") {
		t.Fatalf("first delivery %d %s", st, raw)
	}
	if r := deliverWebhook(ctx, "rt-other", secret, body, ""); r.StatusCode != http.StatusOK {
		t.Fatalf("second delivery under another account answered %d", r.StatusCode)
	} else if raw, _ := io.ReadAll(r.Body); !strings.Contains(string(raw), `"duplicate":true`) {
		t.Fatalf("second delivery under another account was applied again: %s", raw)
	}
	if recorded(t, ctx, "rt-other", "evt_once") {
		t.Fatal("the duplicate wrote a record into a second org")
	}
}

// A replay of an old lifecycle event cannot roll a subscription back: the event
// id was recorded when it first arrived.
func TestWebhookReplayCannotRollBackASubscription(t *testing.T) {
	const secret = "whsec_rollback"
	registerSquare(t, secret)
	ctx := ae.NewContext()
	defer ctx.Close()
	org := webhookOrg(t, ctx, checkout.DefaultBrandSlug(), true)
	seedProviderSubscription(t, org, ctx, "sub_rb", checkout.DefaultBrandSlug()+"/alice")
	ev := func(id, status string, at time.Time) []byte {
		return []byte(fmt.Sprintf(`{"type":"subscription.updated","event_id":%q,"created_at":%q,"data":{"object":{"id":"sub_rb","status":%q}}}`, id, at.UTC().Format(time.RFC3339), status))
	}
	old := ev("evt_rb_old", "active", time.Now().Add(-40*24*time.Hour))
	post(ctx, "/v1/commerce/webhooks/square", secret, old, nil)
	post(ctx, "/v1/commerce/webhooks/square", secret, ev("evt_rb_new", "canceled", time.Now()), nil)
	post(ctx, "/v1/commerce/webhooks/square", secret, old, nil)
	sub := subscription.New(datastore.New(org.Namespaced(ctx)))
	if _, err := sub.Query().Filter("ProviderId=", "sub_rb").Get(); err != nil {
		t.Fatal(err)
	}
	if sub.Status != "canceled" {
		t.Fatalf("status after a replay of the old event = %q, want canceled", sub.Status)
	}
}

// anyBody is a processor whose verifier needs no secret: it accepts whatever is
// posted, as a public-chain lookup or a certificate fetched from a URL the
// payload names effectively does.
type anyBody struct{ processor.PaymentProcessor }

func (anyBody) Type() processor.ProcessorType { return "anybody" }
func (anyBody) ValidateWebhook(context.Context, []byte, string) (*processor.WebhookEvent, error) {
	return &processor.WebhookEvent{ID: "evt_forged", Type: "refund.updated", Processor: "anybody",
		Data: map[string]interface{}{"refund": map[string]interface{}{"id": "rf", "payment_id": "pay", "status": "COMPLETED"}}}, nil
}

// Only a processor that holds its webhook secret may verify a delivery. One that
// needs none — or a Square account with no key configured — is never asked.
func TestWebhookNeedsAProcessorThatHoldsItsSecret(t *testing.T) {
	processor.Global().Register(anyBody{})
	t.Cleanup(func() { processor.Global().Unregister("anybody") })
	ctx := ae.NewContext()
	defer ctx.Close()
	if st, raw := post(ctx, "/v1/commerce/webhooks/anybody", "", []byte(`{}`), map[string]string{"X-Signature": "x"}); st != http.StatusUnauthorized {
		t.Fatalf("a verifier with no secret admitted a delivery: %d %s", st, raw)
	}

	prev, _ := processor.Global().Registered(processor.Square)
	t.Cleanup(func() { processor.Global().Register(prev) })
	processor.Global().Register(squareprovider.NewProvider(squareprovider.Config{AccessToken: "t", LocationID: "l", WebhookURL: testWebhookURL}))
	body := squareEvent("evt_nokey", "sub_nokey", time.Now())
	if st, raw := post(ctx, "/v1/commerce/webhooks/square", "", body, nil); st != http.StatusUnauthorized {
		t.Fatalf("a Square account with no signature key admitted a delivery: %d %s", st, raw)
	}
}

// A refund whose receipt names an invoice these books do not hold cannot be
// cured by redelivery, so it is reconciled by hand and acknowledged, never left
// open for Square to retry for three days.
func TestWebhookMissingInvoiceIsNotRetried(t *testing.T) {
	const secret = "whsec_noinv"
	registerSquare(t, secret)
	ctx := ae.NewContext()
	defer ctx.Close()
	if err := paymentorg.Put("sqpay_noinv", checkout.DefaultBrandSlug(), checkout.DefaultBrandSlug()+"/bob", "inv_absent", false); err != nil {
		t.Fatal(err)
	}
	body := refundEvent("evt_noinv", "refund.updated", "sqrf_noinv", "sqpay_noinv", "COMPLETED", 500)
	if r := deliverWebhook(ctx, "", secret, body, ""); r.StatusCode != http.StatusOK {
		t.Fatalf("a refund on a missing invoice answered %d, want 200 (reconciled, not retried)", r.StatusCode)
	}
}

// An account that belongs to one org reports on its own payments only: a refund
// it verifies, naming a payment whose receipt is another org's, moves nothing.
func TestWebhookClawbackStaysInTheAccountsOrg(t *testing.T) {
	const secret = "whsec_claworg"
	registerSquare(t, secret)
	fake := newFakeLedger()
	injectLedger(t, fake)
	ctx := ae.NewContext()
	defer ctx.Close()
	if err := paymentorg.Put("sqpay_victim", "victim", "victim/bob", "", false); err != nil {
		t.Fatal(err)
	}
	body := refundEvent("evt_claworg", "refund.updated", "sqrf_claworg", "sqpay_victim", "COMPLETED", 500)
	if r := deliverWebhook(ctx, "tenant", secret, body, ""); r.StatusCode != http.StatusOK {
		t.Fatalf("status %d", r.StatusCode)
	}
	fake.mu.Lock()
	n := len(fake.clawbacks)
	fake.mu.Unlock()
	if n != 0 {
		t.Fatalf("a tenant account's refund clawed back another org's payment (%d clawbacks)", n)
	}
}

// The processors that can receive a webhook at all, pinned: each verifies with a
// secret the deployment holds. A provider joining this set has to bring one.
func TestTheWebhookReceiversAreTheSecretHolders(t *testing.T) {
	var got []string
	for _, p := range processor.Global().RegisteredProcessors() {
		if _, ok := p.(processor.WebhookReceiver); ok {
			got = append(got, string(p.Type()))
		}
	}
	sort.Strings(got)
	if strings.Join(got, ",") != "mpc,square" {
		t.Fatalf("webhook receivers = %v, want exactly [mpc square]", got)
	}
}

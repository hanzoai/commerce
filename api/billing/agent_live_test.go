package billing

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/checkoutsession"
	"github.com/hanzoai/commerce/models/paymentmethod"
	"github.com/hanzoai/commerce/payment/processor"
)

// The headless purchase surface, against Stripe's test mode: tokens are minted with Stripe's
// test helper (POST /v1/test_helpers/shared_payment/granted_tokens), revoked with its revoke
// helper, and charged through the same adapter a card is. No live object is created.

const sptVersion = "2026-10-28.preview"

type grantSpec struct {
	method    string // test payment method; default pm_card_visa
	cur       string // default usd
	max       int64
	expiresIn time.Duration
	recurring string // "month" or ""
}

// grant mints a shared payment token granted to the test account.
func (l *liveStripe) grant(g grantSpec) string {
	l.t.Helper()
	if g.method == "" {
		g.method = "pm_card_visa"
	}
	if g.cur == "" {
		g.cur = "usd"
	}
	if g.expiresIn == 0 {
		g.expiresIn = time.Hour
	}
	form := url.Values{"payment_method": {g.method}, "usage_limits[currency]": {g.cur},
		"usage_limits[max_amount]": {strconv.FormatInt(g.max, 10)},
		"usage_limits[expires_at]": {strconv.FormatInt(time.Now().Add(g.expiresIn).Unix(), 10)}}
	if g.recurring != "" {
		form.Set("usage_limits[recurring_interval]", g.recurring)
	}
	out := l.preview("POST", "/test_helpers/shared_payment/granted_tokens", form)
	return str(out["id"])
}

func (l *liveStripe) revoke(token string) {
	l.t.Helper()
	l.preview("POST", "/test_helpers/shared_payment/granted_tokens/"+token+"/revoke", url.Values{})
}

func (l *liveStripe) preview(method, path string, form url.Values) map[string]any {
	l.t.Helper()
	req, _ := http.NewRequest(method, stripeAPIBase+path, strings.NewReader(form.Encode()))
	req.SetBasicAuth(l.sk, "")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Stripe-Version", sptVersion)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		l.t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	var out map[string]any
	_ = json.Unmarshal(b, &out)
	if resp.StatusCode >= 300 {
		l.t.Fatalf("stripe %s %s: %d %v", method, path, resp.StatusCode, out["error"])
	}
	return out
}

// buy opens a checkout for a line and completes it with token under key.
func (l *liveStripe) buy(line ItemIn, token, key string) CompleteOut {
	l.t.Helper()
	v, f := CreateCheckout(*l.ctx, l.org, l.subject, "", []ItemIn{line})
	if f != nil {
		l.t.Fatalf("create checkout: %v", f.Message)
	}
	return l.complete(v.ID, token, key)
}

func (l *liveStripe) complete(id, token, key string) CompleteOut {
	return CompleteCheckout(*l.ctx, l.org, l.subject, "", id, key, PaymentIn{Type: "shared_payment_token", Token: token})
}

func TestLiveStripe_AgentTopupWithATokenCreditsOnce(t *testing.T) {
	l := liveSetup(t)
	v, f := CreateCheckout(*l.ctx, l.org, l.subject, "", []ItemIn{{Kind: "credit", UsdCents: 2500}})
	if f != nil || v.TotalCents != 2500 || v.Status != "open" {
		t.Fatalf("checkout = %+v %v", v, f)
	}
	// The item is changed while it is open, and priced by the server.
	if v, f = UpdateCheckout(*l.ctx, l.org, l.subject, "", v.ID, []ItemIn{{Kind: "credit", UsdCents: 1500}}); f != nil || v.TotalCents != 1500 {
		t.Fatalf("update = %+v %v", v, f)
	}
	tok := l.grant(grantSpec{max: 1500})
	out := l.complete(v.ID, tok, "k1")
	if out.Status != 200 || out.View.Status != "complete" || out.View.Order == nil || out.View.Order.CreditedCents != 1500 {
		t.Fatalf("complete = %d %+v", out.Status, out.View)
	}
	if got := l.balance(); got != 1500 {
		t.Fatalf("balance = %d; want 1500", got)
	}
	// Completing again under the same key is the same order, and charges nothing.
	again := l.complete(v.ID, tok, "k1")
	if again.Status != 200 || again.View.Order == nil || again.View.Order.ID != out.View.Order.ID || l.balance() != 1500 {
		t.Fatalf("replay = %d %+v balance %d", again.Status, again.View, l.balance())
	}
	// Stripe's own report of that payment, delivered again and again, adds nothing.
	pi := l.call("GET", "/payment_intents", url.Values{"limit": {"3"}})["data"].([]any)[0].(map[string]any)
	body := l.succeededEvent(str(pi["id"]))
	for i := 0; i < 3; i++ {
		if status, _ := l.deliver(body, l.hook); status != 200 {
			t.Fatalf("delivery %d = %d", i, status)
		}
	}
	if l.balance() != 1500 {
		t.Fatalf("a replayed webhook moved the balance to %d", l.balance())
	}
	// Another key on a completed checkout is refused.
	if other := l.complete(v.ID, tok, "k2"); other.Status != 409 {
		t.Fatalf("another key = %d; want 409", other.Status)
	}
	// The order reads back, and a token that has been used is not accepted for a new checkout.
	if o, f := ReadOrder(*l.ctx, l.org, l.subject, v.ID); f != nil || o.CreditedCents != 1500 || o.Status != "paid" {
		t.Fatalf("order = %+v %v", o, f)
	}
	used := l.buy(ItemIn{Kind: "credit", UsdCents: 1500}, tok, "k3")
	if used.Status != 402 || used.View.Error == nil || used.View.Error.Code != "token_revoked" || l.balance() != 1500 {
		t.Fatalf("a used token = %d %+v balance %d", used.Status, used.View.Error, l.balance())
	}
}

func TestLiveStripe_AgentRefusesATokenItCannotUse(t *testing.T) {
	l := liveSetup(t)
	line := ItemIn{Kind: "credit", UsdCents: 2500}
	for _, tc := range []struct {
		name, code string
		token      func() string
	}{
		{"over its limit", "token_over_limit", func() string { return l.grant(grantSpec{max: 1000}) }},
		{"in another currency", "token_wrong_currency", func() string { return l.grant(grantSpec{max: 2500, cur: "eur"}) }},
		{"revoked", "token_revoked", func() string { tok := l.grant(grantSpec{max: 2500}); l.revoke(tok); return tok }},
		{"another seller's", "token_wrong_merchant", func() string { return "spt_not_granted_to_this_seller" }},
		{"expired", "token_expired", func() string {
			tok := l.grant(grantSpec{max: 2500, expiresIn: 2 * time.Second})
			time.Sleep(3 * time.Second)
			return tok
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			start := l.balance()
			out := l.buy(line, tc.token(), "refuse-"+tc.code)
			if out.Status != 402 && out.Status != 409 {
				t.Fatalf("status = %d %+v", out.Status, out.View)
			}
			// An expired token may be reported as revoked or expired by Stripe's clock; every
			// other refusal is exactly its code.
			if got := out.View.Error; got == nil || (got.Code != tc.code && !(tc.code == "token_expired" && got.Code == "token_revoked")) {
				t.Fatalf("error = %+v; want %s", got, tc.code)
			}
			if l.balance() != start {
				t.Fatalf("a refused token credited %d", l.balance()-start)
			}
			// The checkout stays usable: the same checkout completes with a good token.
			id := out.View.ID
			good := l.complete(id, l.grant(grantSpec{max: 2500}), "retry-"+tc.code)
			if good.Status != 200 || l.balance() != start+2500 {
				t.Fatalf("retry on the same checkout = %d %+v balance %d", good.Status, good.View, l.balance())
			}
		})
	}
}

func TestLiveStripe_AgentThreeDSecureIsATypedStepNotASuccess(t *testing.T) {
	l := liveSetup(t)
	tok := l.grant(grantSpec{max: 2500, method: "pm_card_authenticationRequired"})
	out := l.buy(ItemIn{Kind: "credit", UsdCents: 2500}, tok, "3ds")
	if out.Status != 402 || out.View.Error == nil || out.View.Error.Code != "authentication_required" || out.View.Next == nil || out.View.Next.Type != "authenticate" || out.View.Next.ClientSecret == "" {
		t.Fatalf("3-D Secure = %d %+v next %+v", out.Status, out.View.Error, out.View.Next)
	}
	if l.balance() != 0 {
		t.Fatalf("an unauthenticated payment credited %d", l.balance())
	}
}

// planPeriod moves a subscription's period into the past so the sweep renews it.
func (l *liveStripe) due(subscriptionID string) {
	l.t.Helper()
	db := datastore.New(l.org.Namespaced(*l.ctx))
	s := reload(l.t, db, subscriptionID)
	shift := time.Until(s.PeriodEnd) + time.Hour
	s.PeriodStart, s.PeriodEnd = s.PeriodStart.Add(-shift), s.PeriodEnd.Add(-shift)
	if err := s.Update(); err != nil {
		l.t.Fatal(err)
	}
}

func TestLiveStripe_AgentSubscriptionRenewsOnARecurringToken(t *testing.T) {
	l := liveSetup(t)
	tok := l.grant(grantSpec{max: 3000, recurring: "month"})
	out := l.buy(ItemIn{Kind: "plan", Plan: "pro-3x"}, tok, "sub-1")
	if out.Status != 200 || out.View.Order == nil || out.View.Order.SubscriptionID == "" {
		t.Fatalf("first charge = %d %+v", out.Status, out.View)
	}
	subID := out.View.Order.SubscriptionID
	db := datastore.New(l.org.Namespaced(*l.ctx))
	if reload(t, db, subID).DefaultPaymentMethod == "" {
		t.Fatal("a recurring token was not kept for the renewals")
	}

	// Renewal success: charged on the token, within its limit.
	l.due(subID)
	if r := renewDue(*l.ctx, l.org, ""); len(r) != 1 || !r[0].Success {
		t.Fatalf("renewal on a recurring token = %+v", r)
	}
	// Its payment_intent.succeeded, delivered again and again, is one entry.
	pi := l.call("GET", "/payment_intents", url.Values{"limit": {"5"}})["data"].([]any)[0].(map[string]any)
	body := l.succeededEvent(str(pi["id"]))
	credits := len(l.ledger.credits())
	for i := 0; i < 3; i++ {
		if status, _ := l.deliver(body, l.hook); status != 200 {
			t.Fatalf("delivery %d = %d", i, status)
		}
	}
	if got := len(l.ledger.credits()); got != credits || l.balance() != 0 {
		t.Fatalf("a replayed renewal webhook credited the wallet: %d credits (was %d), balance %d", got, credits, l.balance())
	}
	if n := l.succeededForSubject(); n != 2 {
		t.Fatalf("Stripe holds %d succeeded payments; want the first period and one renewal", n)
	}

	// Over the token's limit: it has nothing left for a second renewal. The subscription goes
	// past due with a typed code and a path to supply a fresh token.
	l.due(subID)
	before := billingEventsOf(t, *l.ctx, "renewal.action_required")
	if r := renewDue(*l.ctx, l.org, ""); len(r) != 1 || r[0].Success {
		t.Fatalf("renewal over the token's limit = %+v; want a failure", r)
	}
	if n := billingEventsOf(t, *l.ctx, "renewal.action_required"); n != before+1 {
		t.Fatalf("renewal.action_required events = %d; want %d", n, before+1)
	}
	inv := openInvoiceOf(t, db, subID)
	fresh := l.grant(grantSpec{max: 5000, recurring: "month"})
	paid, f := PayInvoiceWithToken(*l.ctx, l.org, l.subject, inv.Id(), fresh)
	if f != nil || paid.Status != PayPaid {
		t.Fatalf("a fresh token for the invoice = %+v %v", paid, f)
	}
	if got := reload(t, db, subID).DefaultPaymentMethod; got == "" {
		t.Fatal("the fresh token was not kept for the next renewal")
	}

	// Revoked between cycles: the next renewal fails with token_revoked and asks for a token.
	l.revoke(fresh)
	l.due(subID)
	before = billingEventsOf(t, *l.ctx, "renewal.action_required")
	if r := renewDue(*l.ctx, l.org, ""); len(r) != 1 || r[0].Success {
		t.Fatalf("renewal on a revoked token = %+v; want a failure", r)
	}
	if n := billingEventsOf(t, *l.ctx, "renewal.action_required"); n <= before {
		t.Fatal("a revoked token announced no renewal action")
	}
}

func TestLiveStripe_AgentOneTimeTokenNeverRenews(t *testing.T) {
	l := liveSetup(t)
	tok := l.grant(grantSpec{max: 1500}) // no recurring interval
	out := l.buy(ItemIn{Kind: "plan", Plan: "pro-3x"}, tok, "once-1")
	if out.Status != 200 || out.View.Order == nil {
		t.Fatalf("first charge = %d %+v", out.Status, out.View)
	}
	subID := out.View.Order.SubscriptionID
	db := datastore.New(l.org.Namespaced(*l.ctx))
	if got := reload(t, db, subID).DefaultPaymentMethod; got != "" {
		t.Fatalf("a one-time token is stored for renewals: %s", got)
	}
	pms := make([]*paymentmethod.PaymentMethod, 0)
	_, _ = paymentmethod.Query(db).Filter("ProviderRef=", tok).GetAll(&pms)
	if len(pms) != 0 {
		t.Fatalf("a one-time token is still stored (%d rows)", len(pms))
	}
	l.due(subID)
	before := billingEventsOf(t, *l.ctx, "renewal.action_required")
	if r := renewDue(*l.ctx, l.org, ""); len(r) != 1 || r[0].Success {
		t.Fatalf("renewal with no token = %+v; want a failure", r)
	}
	if n := billingEventsOf(t, *l.ctx, "renewal.action_required"); n != before+1 {
		t.Fatalf("renewal.action_required events = %d; want %d", n, before+1)
	}
}

func TestLiveStripe_AgentCryptoTopupOpensAStripeDepositWithoutABrowser(t *testing.T) {
	l := liveSetup(t)
	out, f := OpenCryptoTopup(*l.ctx, l.org, CryptoTopupIn{Subject: l.subject, AmountCents: 2500, Network: "base", IdempotencyKey: "crypto-" + l.run})
	if f != nil {
		t.Skipf("this test account takes no stablecoin payments: %v", f.Message)
	}
	if !out.UsdOnly || out.Currency != "usd" || out.AmountCents != 2500 || len(out.Addresses) == 0 || out.Addresses[0].Address == "" {
		t.Fatalf("deposit = %+v", out)
	}
	if p := mustIntent(t, out.IntentID); p.Purpose != "topup" || p.Cents != 2500 || !p.Delayed || p.Done {
		t.Fatalf("the deposit's record = %+v", p)
	}
	if _, f := OpenCryptoTopup(*l.ctx, l.org, CryptoTopupIn{Subject: l.subject, AmountCents: 2500, Network: "bitcoin"}); f == nil || f.Status != 400 {
		t.Fatalf("an unknown network = %v", f)
	}
	// A checkout for credit paid by stablecoin completes when that credit is posted.
	v, _ := CreateCheckout(*l.ctx, l.org, l.subject, "", []ItemIn{{Kind: "credit", UsdCents: 2500}})
	done := CompleteCheckout(*l.ctx, l.org, l.subject, "", v.ID, "crypto-key", PaymentIn{Type: "stablecoin", Network: "solana"})
	if done.Status != 202 || done.View.Next == nil || done.View.Next.Type != "send_deposit" || len(done.View.Next.Addresses) == 0 {
		t.Fatalf("stablecoin checkout = %d %+v", done.Status, done.View)
	}
	// Nothing is credited until the deposit confirms; when Stripe reports it succeeded, it is
	// credited once, however often that is delivered.
	if l.balance() != 0 {
		t.Fatal("an open deposit credited")
	}
	db := datastore.New(l.org.Namespaced(*l.ctx))
	useStripe(t, &stripeDouble{tx: &processor.Transaction{ID: out.IntentID, ProcessorRef: "ch_" + out.IntentID, Amount: 2500, Currency: "usd", Status: "succeeded"}})
	for i := 0; i < 3; i++ {
		if err := applySettlementEvent(*l.ctx, db, l.org, succeeded(out.IntentID, "ch_"+out.IntentID, 2500, "usd")); err != nil {
			t.Fatal(err)
		}
	}
	if l.balance() != 2500 {
		t.Fatalf("balance after the deposit = %d; want 2500 once", l.balance())
	}
	intent := done.View.Next.IntentID
	useStripe(t, &stripeDouble{tx: &processor.Transaction{ID: intent, ProcessorRef: "ch_" + intent, Amount: 2500, Currency: "usd", Status: "succeeded"}})
	if err := applySettlementEvent(*l.ctx, db, l.org, succeeded(intent, "ch_"+intent, 2500, "usd")); err != nil {
		t.Fatal(err)
	}
	got, f := ReadCheckout(*l.ctx, l.org, l.subject, v.ID)
	if f != nil || got.Status != checkoutsession.Complete || got.Order == nil {
		t.Fatalf("the checkout after the deposit = %+v %v", got, f)
	}
}

// openInvoiceOf is the open invoice of a subscription.
func openInvoiceOf(t *testing.T, db *datastore.Datastore, subID string) *billinginvoice.BillingInvoice {
	t.Helper()
	inv := billinginvoice.New(db)
	if found, err := inv.Query().Filter("SubscriptionId=", subID).Filter("Status=", billinginvoice.Open).Get(); err != nil || !found {
		t.Fatalf("the open invoice: found %v err %v", found, err)
	}
	return inv
}

// succeededForSubject counts the succeeded payments Stripe holds for the test's buyer, by the
// subject each carries in its metadata: a token's payments have no customer to list by.
func (l *liveStripe) succeededForSubject() int {
	n := 0
	for _, it := range l.call("GET", "/payment_intents", url.Values{"limit": {"50"}})["data"].([]any) {
		pi := it.(map[string]any)
		md, _ := pi["metadata"].(map[string]any)
		if pi["status"] == "succeeded" && md["hanzo_subject"] == l.subject {
			n++
		}
	}
	return n
}

func TestLiveStripe_AgentIsWarnedBeforeTheTokenRunsOut(t *testing.T) {
	l := liveSetup(t)
	// A token with room for the first period only.
	tok := l.grant(grantSpec{max: 1500, recurring: "month"})
	out := l.buy(ItemIn{Kind: "plan", Plan: "pro-3x"}, tok, "warn-1")
	if out.Status != 200 || out.View.Order == nil {
		t.Fatalf("first charge = %d %+v", out.Status, out.View)
	}
	subID := out.View.Order.SubscriptionID
	// The order and the subscription say how much room is left: none for the next renewal.
	read, _ := ReadCheckout(*l.ctx, l.org, l.subject, out.View.ID)
	if read.Order.Token == nil || read.Order.Token.Covers || read.Order.Token.RemainingCents != 0 || read.Order.Token.RenewalCents != 1500 {
		t.Fatalf("the order's token = %+v", read.Order.Token)
	}
	// The renewal is two days off: the agent is told once, before any attempt.
	db := datastore.New(l.org.Namespaced(*l.ctx))
	s := reload(t, db, subID)
	s.PeriodEnd = time.Now().Add(48 * time.Hour)
	if err := s.Update(); err != nil {
		t.Fatal(err)
	}
	before := billingEventsOf(t, *l.ctx, "renewal.action_required")
	warnTokenRenewals(*l.ctx, l.org)
	warnTokenRenewals(*l.ctx, l.org)
	if n := billingEventsOf(t, *l.ctx, "renewal.action_required"); n != before+1 {
		t.Fatalf("renewal.action_required events = %d; want %d (once, ahead of the attempt)", n, before+1)
	}
	// A fresh token with room covers it.
	room, aerr, f := SupplySubscriptionToken(*l.ctx, l.org, l.subject, subID, l.grant(grantSpec{max: 4500, recurring: "month"}))
	if f != nil || aerr != nil || !room.Covers || room.RemainingCents != 4500 {
		t.Fatalf("a fresh token = %+v %+v %v", room, aerr, f)
	}
	if one := (func() *TokenRoom { return subscriptionRoom(*l.ctx, l.org, subID) })(); one == nil || !one.Covers {
		t.Fatalf("the subscription's token after refresh = %+v", one)
	}
	// A token without the room is refused.
	if _, aerr, _ := SupplySubscriptionToken(*l.ctx, l.org, l.subject, subID, l.grant(grantSpec{max: 1000, recurring: "month"})); aerr == nil || aerr.Code != "token_over_limit" {
		t.Fatalf("a token without room = %+v", aerr)
	}
}

// succeededEvent is Stripe's payment_intent.succeeded for a payment, as the webhook would carry
// it. The payment is retrieved until it reads succeeded (up to 180s); the event is then looked
// for in Stripe's event list for 30s, and when Stripe has not listed it yet the event is built
// from the retrieved payment, which is what the delivery carries. The test then delivers it to
// commerce's webhook as often as it likes: what it asserts is that the credit posts once.
func (l *liveStripe) succeededEvent(id string) []byte {
	l.t.Helper()
	var pi map[string]any
	for deadline := time.Now().Add(180 * time.Second); ; time.Sleep(2 * time.Second) {
		pi = l.call("GET", "/payment_intents/"+id, nil)
		if pi["status"] == "succeeded" {
			break
		}
		if time.Now().After(deadline) {
			l.t.Fatalf("payment %s is %v after 180s; want succeeded", id, pi["status"])
		}
	}
	for deadline := time.Now().Add(30 * time.Second); time.Now().Before(deadline); time.Sleep(2 * time.Second) {
		list := l.call("GET", "/events", url.Values{"type": {"payment_intent.succeeded"}, "limit": {"50"}})
		items, _ := list["data"].([]any)
		for _, it := range items {
			ev, _ := it.(map[string]any)
			data, _ := ev["data"].(map[string]any)
			if obj, _ := data["object"].(map[string]any); str(obj["id"]) == id {
				body, _ := l.bytes("GET", "/events/"+str(ev["id"]), nil)
				return body
			}
		}
	}
	body, _ := json.Marshal(map[string]any{
		"id": "evt_built_" + id, "object": "event", "type": "payment_intent.succeeded", "livemode": false,
		"created": time.Now().Unix(), "data": map[string]any{"object": pi},
	})
	return body
}

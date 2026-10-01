package billing

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/hanzoai/commerce/api/promo"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/test/ae"
)

// What a subscription's period in hand was paid for — its seats and its money —
// read from the payment that paid it, never from the row's live quantity, which the
// buyer can change.

// subscribed buys planID by card for subject and answers the row the sale opened.
func subscribed(t *testing.T, ctx context.Context, org *organization.Organization, subject, planID string, qty int, pr *promo.Promo) *subscription.Subscription {
	t.Helper()
	sold, err := SubscribeCard(ctx, org, SubscribeIn{SourceID: "cnon:ok", PlanID: planID, Subject: subject, Quantity: qty, Promo: pr})
	if err != nil || sold.Sale == nil {
		t.Fatalf("subscribe %s to %s: %+v, %v", subject, planID, sold, err)
	}
	sub, err := loadSubscription(ctx, org, sold.Sale.SubscriptionID)
	if err != nil {
		t.Fatalf("load subscription: %v", err)
	}
	return sub
}

// viewOf is the one row the billing page reads for subject.
func viewOf(t *testing.T, ctx context.Context, org *organization.Organization, subject string) Subscription {
	t.Helper()
	for _, v := range mustSubscriptions(t, ctx, org, subject) {
		if v.ProviderType != "bundle" {
			return v
		}
	}
	t.Fatalf("%s holds no subscription", subject)
	return Subscription{}
}

func mustSubscriptions(t *testing.T, ctx context.Context, org *organization.Organization, subject string) []Subscription {
	t.Helper()
	rows, err := Subscriptions(ctx, org, subject, "")
	if err != nil {
		t.Fatalf("subscriptions of %s: %v", subject, err)
	}
	return rows
}

func paidAs(t *testing.T, v Subscription, settled string, seats int, charged int64) {
	t.Helper()
	if v.Settled != settled || v.Seats != seats || v.ChargedCents != charged {
		t.Fatalf("period reads settled %q, %d seat(s), %d cents; want %q, %d, %d", v.Settled, v.Seats, v.ChargedCents, settled, seats, charged)
	}
}

// A per-seat plan sold at a discount reads the seats and the money its payment
// charged, and a quantity written onto the row afterwards changes neither.
func TestSubscriptions_ThePeriodReadsWhatItsPaymentCharged(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("pp-charged")
	withFakeSquare(t, squareMock("cust_pp", "ccof_pp", "sqpay_pp"))
	sub := subscribed(t, ctx, org, "pp-charged", "team", 3, &promo.Promo{PercentOff: 50, Active: true})

	paidAs(t, viewOf(t, ctx, org, "pp-charged"), "card", 3, 3750)

	sub.Quantity = 9
	if err := sub.Update(); err != nil {
		t.Fatalf("update: %v", err)
	}
	paidAs(t, viewOf(t, ctx, org, "pp-charged"), "card", 3, 3750)
}

// A renewal invoice carries the period's fee and the usage of the period before it;
// the period was charged the fee alone.
func TestSubscriptions_ThePeriodIsChargedItsFeeNotTheUsage(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("pp-usage")
	db := datastore.New(org.Namespaced(ctx))
	sub := seedRow(t, db, "pp-usage", "dev", "square", time.Now().AddDate(0, 0, -2))
	inv := billinginvoice.New(db)
	inv.UserId, inv.SubscriptionId = sub.UserId, sub.Id()
	inv.PeriodStart, inv.PeriodEnd = sub.PeriodStart, sub.PeriodEnd
	inv.Currency = currency.USD
	inv.LineItems = []billinginvoice.LineItem{
		{Id: "li_plan", Type: billinginvoice.LineSubscription, Quantity: 1, UnitPrice: 2000, Amount: 2000},
		{Id: "li_use", Type: billinginvoice.LineUsage, Quantity: 7, UnitPrice: 100, Amount: 700},
	}
	inv.RecalculateSubtotal()
	if err := inv.Finalize(); err != nil {
		t.Fatalf("finalize: %v", err)
	}
	if err := inv.MarkPaid("card", "sqpay_use"); err != nil {
		t.Fatalf("pay: %v", err)
	}
	if err := inv.Create(); err != nil {
		t.Fatalf("create invoice: %v", err)
	}
	paidAs(t, viewOf(t, ctx, org, "pp-usage"), "card", 1, 2000)
}

// A plan recorded as paid outside commerce reads the seats and the price recorded
// with its period, and both are kept with the period.
func TestSubscriptions_AnExternalPeriodReadsWhatWasRecorded(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("pp-ext")
	db := datastore.New(org.Namespaced(ctx))
	in := recordDev(t, db)
	in.Subject, in.PlanID, in.Quantity, in.PriceCents = "pp-ext", "team", 3, 7500
	if _, err := RecordSubscription(ctx, org, in); err != nil {
		t.Fatalf("record: %v", err)
	}
	paidAs(t, viewOf(t, ctx, org, "pp-ext"), "external:square", 3, 7500)
	periods, _ := subsOf(t, db, "pp-ext")[0].Metadata["periods"].([]interface{})
	last, _ := periods[len(periods)-1].(map[string]interface{})
	if q, _ := numberField(last, "quantity"); q != 3 {
		t.Fatalf("period metadata %#v; want its quantity", last)
	}
	if c, _ := numberField(last, "priceCents"); c != 7500 {
		t.Fatalf("period metadata %#v; want its price", last)
	}

	flat := recordDev(t, db)
	flat.Subject = "pp-ext-flat"
	if _, err := RecordSubscription(ctx, org, flat); err != nil {
		t.Fatalf("record flat: %v", err)
	}
	paidAs(t, viewOf(t, ctx, org, "pp-ext-flat"), "external:square", 1, flat.PriceCents)
}

// A period recorded without its seats and price reads what the record checked: the
// row's seats at the plan's price.
func TestSubscriptions_ALegacyExternalPeriodReadsTheRow(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("pp-legacy")
	db := datastore.New(org.Namespaced(ctx))
	in := recordDev(t, db)
	in.Subject, in.PlanID, in.Quantity, in.PriceCents = "pp-legacy", "team", 3, 7500
	got, err := RecordSubscription(ctx, org, in)
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	sub, err := loadSubscription(ctx, org, got.Subscription.ID)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	periods, _ := sub.Metadata["periods"].([]interface{})
	for _, p := range periods {
		m, _ := p.(map[string]interface{})
		delete(m, "quantity")
		delete(m, "priceCents")
	}
	if err := sub.Update(); err != nil {
		t.Fatalf("update: %v", err)
	}
	paidAs(t, viewOf(t, ctx, org, "pp-legacy"), "external:square", 3, 7500)
}

// A row nobody paid for has no seats and was charged nothing.
func TestSubscriptions_AnUnpaidPeriodReadsNoSeats(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("pp-unpaid")
	if w := invokeSub(org, ctx, c1MintPrincipal, CreateBillingSubscription, `{"userId":"pp-unpaid","planId":"team","quantity":4}`); w.StatusCode != http.StatusCreated {
		t.Fatalf("create: %d %s", w.StatusCode, bodyOf(w))
	}
	paidAs(t, viewOf(t, ctx, org, "pp-unpaid"), "", 0, 0)
}

func refundEvent(eventID, typ, refundID, paymentID, status string, cents int64) []byte {
	return []byte(fmt.Sprintf(`{"merchant_id":"M1","type":%q,"event_id":%q,"created_at":%q,`+
		`"data":{"type":"refund","id":%q,"object":{"refund":{"id":%q,"payment_id":%q,"status":%q,`+
		`"amount_money":{"amount":%d,"currency":"USD"}}}}}`, typ, eventID, time.Now().UTC().Format(time.RFC3339), refundID, refundID, paymentID, status, cents))
}

func disputeEvent(eventID, typ, disputeID, paymentID, state string, cents int64) []byte {
	return []byte(fmt.Sprintf(`{"merchant_id":"M1","type":%q,"event_id":%q,"created_at":%q,`+
		`"data":{"type":"dispute","id":%q,"object":{"dispute":{"id":%q,"state":%q,"disputed_payment":{"payment_id":%q},`+
		`"amount_money":{"amount":%d,"currency":"USD"}}}}}`, typ, eventID, time.Now().UTC().Format(time.RFC3339), disputeID, disputeID, state, paymentID, cents))
}

func deliver(t *testing.T, ctx ae.Context, secret string, bodies ...[]byte) {
	t.Helper()
	for _, b := range bodies {
		if r := deliverWebhook(ctx, "", secret, b, ""); r.StatusCode != http.StatusOK {
			t.Fatalf("delivery: %d %s", r.StatusCode, bodyOf(r))
		}
	}
}

// A refund of the payment that paid a plan's period takes that money off what the
// period was charged, once per refund; refunded in full, the period reads unpaid. The
// payment paid a plan and credited no wallet, so no wallet is touched.
func TestWebhook_ARefundOfAPlanPaymentUnpaysItsPeriod(t *testing.T) {
	const secret = "whsec_pp_refund"
	registerSquare(t, secret)
	fake := newFakeLedger()
	injectLedger(t, fake)
	ctx := ae.NewContext()
	defer ctx.Close()
	org := webhookOrg(t, ctx, "pp-refund", true)
	withFakeSquare(t, squareMock("cust_rf", "ccof_rf", "sqpay_rf"))
	subscribed(t, ctx, org, "pp-refund", "team", 2, nil)
	paidAs(t, viewOf(t, ctx, org, "pp-refund"), "card", 2, 5000)

	deliver(t, ctx, secret,
		refundEvent("evt_pp_rf_0", "refund.created", "rf_a", "sqpay_rf", "PENDING", 2000),
		refundEvent("evt_pp_rf_1", "refund.updated", "rf_a", "sqpay_rf", "COMPLETED", 2000),
		refundEvent("evt_pp_rf_2", "refund.updated", "rf_a", "sqpay_rf", "COMPLETED", 2000))
	paidAs(t, viewOf(t, ctx, org, "pp-refund"), "card", 2, 3000)

	deliver(t, ctx, secret, refundEvent("evt_pp_rf_3", "refund.updated", "rf_b", "sqpay_rf", "COMPLETED", 3000))
	paidAs(t, viewOf(t, ctx, org, "pp-refund"), "", 0, 0)

	if len(fake.clawbacks) != 0 {
		t.Fatalf("clawbacks = %+v; a plan payment credited no wallet to take back from", fake.clawbacks)
	}
}

// A renewal's card payment is found by its refund the same way the first one is.
func TestWebhook_ARefundOfARenewalUnpaysItsPeriod(t *testing.T) {
	const secret = "whsec_pp_renewal"
	registerSquare(t, secret)
	ctx := ae.NewContext()
	defer ctx.Close()
	org := webhookOrg(t, ctx, "pp-renewal", true)
	withFakeSquare(t, squareMock("", "", "sqpay_rn"))
	sub := seedCardBackedSub(t, datastore.New(org.Namespaced(ctx)), "pp-renewal", "dev", "ccof_rn", "cust_rn")
	if r := invokeRenew(org, ctx, sub.Id()); r.StatusCode != http.StatusOK {
		t.Fatalf("renew: %d %s", r.StatusCode, bodyOf(r))
	}
	price := lookupPlan("dev").Price
	paidAs(t, viewOf(t, ctx, org, "pp-renewal"), "card", 1, price)

	deliver(t, ctx, secret, refundEvent("evt_pp_rn_0", "refund.updated", "rf_rn", "sqpay_rn", "COMPLETED", price))
	paidAs(t, viewOf(t, ctx, org, "pp-renewal"), "", 0, 0)
}

// A dispute of the payment that paid a plan's period leaves the period unpaid while
// it is open and once it is lost; a dispute won restores it.
func TestWebhook_ADisputeOfAPlanPaymentUnpaysItsPeriodUntilWon(t *testing.T) {
	const secret = "whsec_pp_dispute"
	registerSquare(t, secret)
	ctx := ae.NewContext()
	defer ctx.Close()
	org := webhookOrg(t, ctx, "pp-dispute", true)
	price := lookupPlan("dev").Price

	withFakeSquare(t, squareMock("cust_lost", "ccof_lost", "sqpay_lost"))
	subscribed(t, ctx, org, "pp-dispute-lost", "dev", 1, nil)
	withFakeSquare(t, squareMock("cust_won", "ccof_won", "sqpay_won"))
	subscribed(t, ctx, org, "pp-dispute-won", "dev", 1, nil)

	deliver(t, ctx, secret,
		disputeEvent("evt_pp_dl_0", "dispute.created", "dp_lost", "sqpay_lost", "EVIDENCE_REQUIRED", price),
		disputeEvent("evt_pp_dw_0", "dispute.created", "dp_won", "sqpay_won", "EVIDENCE_REQUIRED", price))
	paidAs(t, viewOf(t, ctx, org, "pp-dispute-lost"), "", 0, 0)
	paidAs(t, viewOf(t, ctx, org, "pp-dispute-won"), "", 0, 0)

	deliver(t, ctx, secret,
		disputeEvent("evt_pp_dl_1", "dispute.state.changed", "dp_lost", "sqpay_lost", "LOST", price),
		disputeEvent("evt_pp_dw_1", "dispute.state.changed", "dp_won", "sqpay_won", "WON", price),
		disputeEvent("evt_pp_dw_2", "dispute.updated", "dp_won", "sqpay_won", "PROCESSING", price))
	paidAs(t, viewOf(t, ctx, org, "pp-dispute-lost"), "", 0, 0)
	paidAs(t, viewOf(t, ctx, org, "pp-dispute-won"), "card", 1, price)
}

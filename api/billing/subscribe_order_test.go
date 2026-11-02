package billing

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/plan"
	"github.com/hanzoai/commerce/models/transaction/util"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/test/ae"
)

// An organization's order (THE SEAT PLAN AN ORGANIZATION BUYS, AND ITS STARTING
// BALANCE, IN ONE PAYMENT):
//
//	its seats are the ROW's: per seat, floor and ceiling as the authority row
//	                         states them, so a restated row moves the sale
//	its starting balance:    whole dollars, at most the row's balanceCeiling,
//	                         charged with the seats and credited to the org's
//	                         prepaid balance; refused before the card is touched
//	its buyer:               the organization's own account, never a wallet
//	its receipt:             one payment: amountCents = seats x year + balance

func orderRow(t *testing.T, ctx context.Context, slug string, minSeats, maxMembers int) *plan.Plan {
	t.Helper()
	n := func(v int) *int { return &v }
	row := plan.New(plan.AuthorityDB(ctx))
	row.Slug, row.Name, row.Category, row.Managed = slug, "Enterprise", "enterprise", true
	row.Price, row.AnnualTotal, row.PriceAnnual = 24000, 24000, 2000
	row.Currency, row.Interval, row.IntervalCount, row.PerSeat = currency.USD, "year", 1, true
	row.Limits = &plan.Limits{
		MinSeats: n(minSeats), MaxMembers: n(maxMembers),
		SpendLimits: []int{20, 50, 100, 200, 500, 1000}, SpendLimit: n(100),
		BalanceMonths: n(1), BalanceCeiling: n(50000),
	}
	if err := row.Create(); err != nil {
		t.Fatalf("create %s: %v", slug, err)
	}
	return row
}

func bodyText(r *http.Response) string { b, _ := io.ReadAll(r.Body); return string(b) }

// TestSubscribeWithCard_AnOrderIsRefusedBeforeTheCard: every order the row does not
// sell is a 400 with nothing charged and nothing opened — seats outside the row's
// floor and ceiling, a balance that is not whole dollars, over the ceiling or
// negative, a balance paid from prepaid money or a wallet.
func TestSubscribeWithCard_AnOrderIsRefusedBeforeTheCard(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("so-refused")
	m := squareMock("cust_o", "ccof_o", "sqpay_o")
	withFakeSquare(t, m)
	orderRow(t, ctx, "so-ent-refused", 20, 500)

	for _, body := range []string{
		`{"sourceId":"cnon:ok","planId":"so-ent-refused","quantity":19,"interval":"year"}`,
		`{"sourceId":"cnon:ok","planId":"so-ent-refused","quantity":501,"interval":"year"}`,
		`{"sourceId":"cnon:ok","planId":"so-ent-refused","quantity":20,"interval":"year","balanceCents":150}`,
		`{"sourceId":"cnon:ok","planId":"so-ent-refused","quantity":20,"interval":"year","balanceCents":-100}`,
		`{"sourceId":"cnon:ok","planId":"so-ent-refused","quantity":20,"interval":"year","balanceCents":5000100}`,
		`{"sourceId":"balance","planId":"so-ent-refused","quantity":20,"interval":"year","balanceCents":10000}`,
		`{"sourceId":"wallet:tok","wallet":true,"planId":"so-ent-refused","quantity":20,"interval":"year","balanceCents":10000}`,
	} {
		if r := invokeSubscribeCard(org, ctx, body, nil); r.StatusCode != http.StatusBadRequest {
			t.Fatalf("%s: status=%d body=%s, want 400", body, r.StatusCode, bodyText(r))
		}
	}
	if m.chargeCalls != 0 {
		t.Fatalf("charge calls=%d for orders the row does not sell, want 0", m.chargeCalls)
	}
	if subs := subsOf(t, datastore.New(org.Namespaced(ctx)), "so-refused"); len(subs) != 0 {
		t.Fatalf("refused orders opened %d subscriptions", len(subs))
	}
}

// TestSubscribe_AnOrderIsTheOrganizations: a row that states how an organization
// orders it is never sold to a member's wallet, whoever resolved the subject.
func TestSubscribe_AnOrderIsTheOrganizations(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("so-wallet")
	m := squareMock("cust_w", "ccof_w", "sqpay_w")
	withFakeSquare(t, m)
	orderRow(t, ctx, "so-ent-wallet", 20, 500)

	_, err := Subscribe(ctx, org, SubscribeIn{
		SourceID: "cnon:ok", PlanID: "so-ent-wallet", Subject: "so-wallet/bob", Quantity: 20, Interval: "year",
	})
	if !IsSaleRefused(err) {
		t.Fatalf("an organization's plan bought for a member's wallet: err=%v, want a refusal", err)
	}
	if m.chargeCalls != 0 {
		t.Fatalf("charge calls=%d, want 0", m.chargeCalls)
	}
}

// TestSubscribeWithCard_AnOrderIsOnePayment: twenty seats for a year and a $400
// starting balance are one charge of $5,200. The invoice holds the seats, the
// balance holds the $400 under the payment's reference, the receipt says both, and
// a retry under the same key replays the receipt without charging again.
func TestSubscribeWithCard_AnOrderIsOnePayment(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("so-paid")
	m := squareMock("cust_p", "ccof_p", "sqpay_p")
	withFakeSquare(t, m)
	orderRow(t, ctx, "so-ent-paid", 20, 500)

	body := `{"sourceId":"cnon:ok","planId":"so-ent-paid","quantity":20,"interval":"year","balanceCents":40000}`
	key := map[string]string{"X-Idempotency-Key": "order-1"}
	r := invokeSubscribeCard(org, ctx, body, key)
	if r.StatusCode != http.StatusCreated {
		t.Fatalf("status=%d body=%s, want 201", r.StatusCode, bodyText(r))
	}
	raw, _ := io.ReadAll(r.Body)
	var sale Sale
	if err := json.Unmarshal(raw, &sale); err != nil {
		t.Fatalf("receipt: %v (%s)", err, raw)
	}
	if m.chargeCalls != 1 || m.lastChargeAmount != 520000 {
		t.Fatalf("charged %d time(s), last %d; want one charge of 520000 (20 x 24000 + 40000)", m.chargeCalls, m.lastChargeAmount)
	}
	if sale.AmountCents != 520000 || sale.BalanceCents != 40000 || sale.BalanceID == "" ||
		sale.Seats != 20 || sale.Category != "enterprise" || sale.Interval != "year" || sale.Replaced != "" {
		t.Fatalf("receipt %+v; want amount 520000, balance 40000 with its id, 20 seats, enterprise, year, nothing replaced", sale)
	}

	db := datastore.New(org.Namespaced(ctx))
	sub := parentSub(t, db, "so-paid", "so-ent-paid")
	if sub == nil || sub.Quantity != 20 || !sub.Plan.PerSeat {
		t.Fatalf("subscription %+v; want 20 seats of a per-seat plan", sub)
	}
	invs := invoicesForSub(t, db, sub.Id())
	if len(invs) != 1 || invs[0].AmountDue != 480000 || invs[0].AmountPaid != 480000 {
		t.Fatalf("invoices %d; want one paid invoice of 480000 for the seats", len(invs))
	}
	datas, err := util.GetTransactionsByCurrency(org.Namespaced(ctx), "so-paid", "iam-user", currency.USD, org.TestMode())
	if err != nil {
		t.Fatalf("balance: %v", err)
	}
	if got := datas.Data[currency.USD]; got == nil || got.Balance != 40000 {
		t.Fatalf("prepaid balance %+v, want 40000", got)
	}

	again := invokeSubscribeCard(org, ctx, body, key)
	if again.StatusCode != http.StatusOK {
		t.Fatalf("retry status=%d body=%s, want 200 (replay)", again.StatusCode, bodyText(again))
	}
	if replay, _ := io.ReadAll(again.Body); string(replay) != string(raw) {
		t.Fatalf("replay %s, want the first receipt %s", replay, raw)
	}
	if m.chargeCalls != 1 {
		t.Fatalf("a replay charged again: %d charges", m.chargeCalls)
	}
}

// TestSubscribeWithCard_TheRowMovesTheSale: the floor is the row's, so a SuperAdmin
// restating it moves what a card may buy without a release, and a seat plan the
// embed does not sell by the seat is still charged per seat when its row says so.
func TestSubscribeWithCard_TheRowMovesTheSale(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("so-moved")
	m := squareMock("cust_m", "ccof_m", "sqpay_m")
	withFakeSquare(t, m)
	row := orderRow(t, ctx, "so-ent-moved", 20, 500)

	body := `{"sourceId":"cnon:ok","planId":"so-ent-moved","quantity":10,"interval":"year"}`
	if r := invokeSubscribeCard(org, ctx, body, nil); r.StatusCode != http.StatusBadRequest {
		t.Fatalf("10 seats under a floor of 20: status=%d body=%s, want 400", r.StatusCode, bodyText(r))
	}
	ten := 10
	row.Limits.MinSeats = &ten
	if err := row.Update(); err != nil {
		t.Fatalf("restate: %v", err)
	}
	if r := invokeSubscribeCard(org, ctx, body, nil); r.StatusCode != http.StatusCreated {
		t.Fatalf("10 seats once the row's floor is 10: status=%d body=%s, want 201", r.StatusCode, bodyText(r))
	}
	if m.lastChargeAmount != 240000 {
		t.Fatalf("charged %d, want 240000 (10 x 24000)", m.lastChargeAmount)
	}
}

// TestPlanView_ServesTheOrder: the four figures an organization orders by ride the
// public catalog under limits, in whole dollars, as the row states them.
func TestPlanView_ServesTheOrder(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	orderRow(t, ctx, "so-ent-view", 20, 500)
	row := plan.New(plan.AuthorityDB(ctx))
	if ok, err := row.Query().Filter("Slug=", "so-ent-view").Get(); err != nil || !ok {
		t.Fatalf("read the row back: ok=%v err=%v", ok, err)
	}
	b, err := json.Marshal(staticPlanFromModel(row).view())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got struct {
		AnnualTotal int64 `json:"annualTotal"`
		PerSeat     bool  `json:"perSeat"`
		Limits      struct {
			MinSeats       int   `json:"minSeats"`
			MaxMembers     int   `json:"maxMembers"`
			SpendLimits    []int `json:"spendLimits"`
			SpendLimit     int   `json:"spendLimit"`
			BalanceMonths  int   `json:"balanceMonths"`
			BalanceCeiling int   `json:"balanceCeiling"`
		} `json:"limits"`
	}
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	l := got.Limits
	if got.AnnualTotal != 24000 || !got.PerSeat || l.MinSeats != 20 || l.MaxMembers != 500 || len(l.SpendLimits) != 6 ||
		l.SpendLimits[5] != 1000 || l.SpendLimit != 100 || l.BalanceMonths != 1 || l.BalanceCeiling != 50000 {
		t.Fatalf("served %s", b)
	}
}

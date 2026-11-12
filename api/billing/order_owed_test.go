package billing

// An order's starting balance while it is owed: what happened to its payment since
// the charge decides what is credited, and the plan's reload waits for the credit.
// The probes red wrote against commerce#13 (TestRed13_*), adopted.

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"sync"
	"testing"

	"github.com/hanzoai/commerce/datastore"
	dbpkg "github.com/hanzoai/commerce/db"
	"github.com/hanzoai/commerce/models/idempotencykey"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/plan"
	"github.com/hanzoai/commerce/models/transaction"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/util/test/ae"
)

// depositsOf counts the deposits payment ref made to subject in commerce's books.
func depositsOf(t *testing.T, db *datastore.Datastore, subject, ref string) int {
	t.Helper()
	rows := make([]*transaction.Transaction, 0)
	if _, err := transaction.Query(db).Filter("SourceId=", ref).Filter("DestinationId=", subject).GetAll(&rows); err != nil {
		t.Fatalf("deposits: %v", err)
	}
	n := 0
	for _, r := range rows {
		if r.Type == transaction.Deposit {
			n++
		}
	}
	return n
}

// owe sells an order whose starting balance is left owed, the receipt sealed owing
// it, by one of the two things that really leave it so. undecided: a refund of the
// payment is being recorded when the sale credits (the test holds the payment's
// lease), so the credit is not decided and nothing is credited anywhere (409).
// Otherwise: the host ledger refuses the credit once, after commerce's deposit
// landed, so the credit is decided and not landed in the spendable ledger (503).
func owe(t *testing.T, ctx ae.Context, name, payment string, balance int64, undecided bool) (*organization.Organization, *fakeLedger, map[string]string, string) {
	t.Helper()
	fake := &flakyLedger{fakeLedger: newFakeLedger()}
	want := http.StatusConflict
	if !undecided {
		fake.failures, want = 1, http.StatusServiceUnavailable
	}
	injectLedger(t, fake)
	org := webhookOrg(t, ctx, name, true)
	withFakeSquare(t, squareMock("cust_"+name, "ccof_"+name, payment))
	// The plan states a reload, which a sale whose payment was taken back never
	// switches on.
	row := orderRow(t, ctx, name+"-plan", 20, 500)
	row.Limits.Reload = &plan.Reload{ThresholdCents: 1000, AmountCents: 5000}
	if err := row.Update(); err != nil {
		t.Fatal(err)
	}
	body := `{"sourceId":"cnon:ok","planId":"` + name + `-plan","quantity":20,"interval":"year","balanceCents":` + strconv.FormatInt(balance, 10) + `}`
	key := map[string]string{"X-Idempotency-Key": name}
	var lease *idempotencykey.Lease
	if undecided {
		l, ok, err := idempotencykey.Hold(datastore.New(org.Namespaced(ctx)), paymentScope, payment)
		if err != nil || !ok {
			t.Fatalf("hold the payment: %v %v", ok, err)
		}
		lease = l
	}
	r := invokeSubscribeCard(org, ctx, body, key)
	lease.Release()
	if r.StatusCode != want {
		t.Fatalf("order: %d %s, want the credit left owed (%d)", r.StatusCode, bodyText(r), want)
	}
	if b, _ := fake.Balance(ctx, ledgerOrg(org), name, "usd", org.TestMode()); b != 0 {
		t.Fatalf("setup: the owed credit holds %d in the host ledger", b)
	}
	if undecided && balanceOf(t, ctx, org, name) != 0 {
		t.Fatalf("setup: the undecided credit holds %d in commerce's books", balanceOf(t, ctx, org, name))
	}
	return org, fake.fakeLedger, key, body
}

// receiptOf reads a sale's answer.
func receiptOf(t *testing.T, r *http.Response) Sale {
	t.Helper()
	var sale Sale
	if err := json.Unmarshal([]byte(bodyText(r)), &sale); err != nil {
		t.Fatalf("receipt: %v", err)
	}
	return sale
}

// A payment refunded in full while its balance is owed credits nothing when the
// order is sent again: the refund is recorded against the balance, and the credit
// nets against it. Before, the refund took nothing (no deposit yet) and the retry
// credited all of a refunded payment.
func TestRed13_RefundWhileOwedThenCompleteCreditsARefundedPayment(t *testing.T) {
	const secret = "whsec_r13l"
	registerSquare(t, secret)
	ctx := ae.NewContext()
	defer ctx.Close()
	org, fake, key, body := owe(t, ctx, "r13l", "sqpay_r13l", 4000000, true)
	// The whole payment: 20 seats x 24000, and the balance.
	deliver(t, ctx, secret, refundEvent("evt_r13l", "refund.updated", "rf_r13l", "sqpay_r13l", "COMPLETED", 4480000))
	r := invokeSubscribeCard(org, ctx, body, key)
	held, _ := fake.Balance(ctx, ledgerOrg(org), "r13l", "usd", org.TestMode())
	if r.StatusCode != http.StatusCreated || held != 0 || balanceOf(t, ctx, org, "r13l") != 0 || len(fake.credits()) != 0 {
		t.Fatalf("FINDING: a payment refunded in full was credited by the retry: %d %s; host %d (credits %d), commerce %d",
			r.StatusCode, bodyText(r), held, len(fake.credits()), balanceOf(t, ctx, org, "r13l"))
	}
	if sale := receiptOf(t, r); sale.BalanceCents != 0 || sale.BalanceID != "" || sale.Reload != nil {
		t.Fatalf("the receipt of a refunded balance should credit and owe nothing: %+v", sale)
	}
	// The seats were refunded too, beyond the balance.
	db := datastore.New(org.Namespaced(ctx))
	invs := invoicesForSub(t, db, parentSub(t, db, "r13l", "r13l-plan").Id())
	if len(invs) != 1 || invs[0].Refunded() != 480000 {
		t.Fatalf("the seats' invoice should hold the rest of the refund: %+v", invs)
	}
	if len(fake.clawbacks) != 0 {
		t.Fatalf("a take recorded while owed posted a clawback against money never credited: %+v", fake.clawbacks)
	}
	// Replayed, the resolved receipt stays resolved; the key sent with another
	// balance is another order.
	if again := invokeSubscribeCard(org, ctx, body, key); again.StatusCode != http.StatusOK || balanceOf(t, ctx, org, "r13l") != 0 {
		t.Fatalf("replay: %d, balance %d", again.StatusCode, balanceOf(t, ctx, org, "r13l"))
	}
	other := `{"sourceId":"cnon:ok","planId":"r13l-plan","quantity":20,"interval":"year","balanceCents":5000000}`
	if again := invokeSubscribeCard(org, ctx, other, key); again.StatusCode != http.StatusConflict {
		t.Fatalf("the key sent with another balance answered %d %s, want 409", again.StatusCode, bodyText(again))
	}
}

// A refund of the whole payment while the credit is decided and has not landed in
// the spendable ledger: the refund lands the credit and takes it back, in both
// books, and the retry credits nothing again.
func TestARefundWhileTheCreditIsLandingTakesItAll(t *testing.T) {
	const secret = "whsec_owed_land"
	registerSquare(t, secret)
	ctx := ae.NewContext()
	defer ctx.Close()
	org, fake, key, body := owe(t, ctx, "owed-land", "sqpay_owed_land", 4000000, false)
	deliver(t, ctx, secret, refundEvent("evt_owed_land", "refund.updated", "rf_owed_land", "sqpay_owed_land", "COMPLETED", 4480000))
	r := invokeSubscribeCard(org, ctx, body, key)
	held, _ := fake.Balance(ctx, ledgerOrg(org), "owed-land", "usd", org.TestMode())
	if r.StatusCode != http.StatusCreated || held != 0 || balanceOf(t, ctx, org, "owed-land") != 0 || len(fake.credits()) != 1 {
		t.Fatalf("FINDING: %d %s; host %d (credits %d), commerce %d, want everything taken back",
			r.StatusCode, bodyText(r), held, len(fake.credits()), balanceOf(t, ctx, org, "owed-land"))
	}
	if sale := receiptOf(t, r); sale.BalanceID == "" || sale.Reload != nil {
		t.Fatalf("receipt %+v: want the credit's receipt and no reload for a refunded sale", sale)
	}
}

// A refund of part of the balance while it is owed is netted: the retry credits
// the rest, and the reload a refunded sale stopped stays off. A refund after the
// credit takes from the wallet, once however often it is delivered.
func TestAPartialRefundWhileOwedIsNettedFromTheCredit(t *testing.T) {
	const secret = "whsec_owed_part"
	registerSquare(t, secret)
	ctx := ae.NewContext()
	defer ctx.Close()
	org, fake, key, body := owe(t, ctx, "owed-part", "sqpay_owed_part", 4000000, true)
	deliver(t, ctx, secret, refundEvent("evt_owed_part", "refund.updated", "rf_owed_part", "sqpay_owed_part", "COMPLETED", 1000000))
	r := invokeSubscribeCard(org, ctx, body, key)
	held, _ := fake.Balance(ctx, ledgerOrg(org), "owed-part", "usd", org.TestMode())
	if r.StatusCode != http.StatusCreated || held != 3000000 || balanceOf(t, ctx, org, "owed-part") != 3000000 {
		t.Fatalf("FINDING: %d %s; host %d, commerce %d, want 3000000 in each", r.StatusCode, bodyText(r), held, balanceOf(t, ctx, org, "owed-part"))
	}
	if sale := receiptOf(t, r); sale.BalanceCents != 3000000 || sale.BalanceID == "" || sale.Reload != nil {
		t.Fatalf("receipt %+v: want the 3000000 credited and no reload switched on for a refunded sale", sale)
	}
	if rule := loadAutoRecharge(datastore.New(org.Namespaced(ctx)), org.Name); rule != nil && rule.Enabled {
		t.Fatalf("FINDING: completing a refunded order switched its reload on: %+v", rule)
	}
	deliver(t, ctx, secret, refundEvent("evt_owed_part2", "refund.updated", "rf_owed_part2", "sqpay_owed_part", "COMPLETED", 500000))
	deliver(t, ctx, secret, refundEvent("evt_owed_part3", "refund.updated", "rf_owed_part2", "sqpay_owed_part", "COMPLETED", 500000))
	held, _ = fake.Balance(ctx, ledgerOrg(org), "owed-part", "usd", org.TestMode())
	if held != 2500000 || balanceOf(t, ctx, org, "owed-part") != 2500000 {
		t.Fatalf("after a second refund: host %d, commerce %d, want 2500000", held, balanceOf(t, ctx, org, "owed-part"))
	}
}

// One refund delivered many times at once — a processor's retries crossing — on
// an order whose balance is credited takes its part once, in both books. Six
// orders, eight crossing deliveries each.
func TestOneRefundDeliveredTogetherTakesOnce(t *testing.T) {
	const secret = "whsec_rf_dup"
	registerSquare(t, secret)
	fake := newFakeLedger()
	injectLedger(t, fake)
	ctx := ae.NewContext()
	defer ctx.Close()
	orderRow(t, ctx, "rf-dup-plan", 20, 500)
	for n := range 6 {
		name, payment := "rf-dup-"+strconv.Itoa(n), "sqpay_rf_dup_"+strconv.Itoa(n)
		org := webhookOrg(t, ctx, name, true)
		withFakeSquare(t, squareMock("cust_"+name, "ccof_"+name, payment))
		if r := invokeSubscribeCard(org, ctx, `{"sourceId":"cnon:ok","planId":"rf-dup-plan","quantity":20,"interval":"year","balanceCents":4000000}`,
			map[string]string{"X-Idempotency-Key": name}); r.StatusCode != http.StatusCreated {
			t.Fatalf("order: %d %s", r.StatusCode, bodyText(r))
		}
		var wg sync.WaitGroup
		for i := range 8 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				b := refundEvent("evt_"+name+"_"+strconv.Itoa(i), "refund.updated", "rf_"+name, payment, "COMPLETED", 1000000)
				if r := deliverWebhook(ctx, "", secret, b, ""); r.StatusCode != http.StatusOK {
					t.Errorf("%s delivery %d: %d %s", name, i, r.StatusCode, bodyText(r))
				}
			}()
		}
		wg.Wait()
		held, _ := fake.Balance(ctx, ledgerOrg(org), name, "usd", org.TestMode())
		if held != 3000000 || balanceOf(t, ctx, org, name) != 3000000 {
			t.Fatalf("FINDING: %s: one refund of 1000000 delivered 8 times left host %d, commerce %d, want 3000000", name, held, balanceOf(t, ctx, org, name))
		}
	}
}

// A dispute lost while the balance is owed, in the production shape: the delivery
// lands (no clawback against a payment the host never credited, so no 503 loop).
// The retry decides the credit, lands it and takes the withheld part back from the
// wallet in the same step — one clawback, posted against money the host holds — so
// nothing of it is spendable. Won after the credit, that clawback is restored, once.
func TestRed13_DisputeWhileOwedHostLedger(t *testing.T) {
	const secret = "whsec_r13m"
	registerSquare(t, secret)
	ctx := ae.NewContext()
	defer ctx.Close()
	org, fake, key, body := owe(t, ctx, "r13m", "sqpay_r13m", 4000000, true)
	if d := deliverWebhook(ctx, "", secret, disputeEvent("evt_r13m", "dispute.created", "dp_r13m", "sqpay_r13m", "LOST", 4480000), ""); d.StatusCode != http.StatusOK {
		t.Fatalf("FINDING: the lost dispute while owed answered %d %s", d.StatusCode, bodyText(d))
	}
	r := invokeSubscribeCard(org, ctx, body, key)
	held, _ := fake.Balance(ctx, ledgerOrg(org), "r13m", "usd", org.TestMode())
	if r.StatusCode != http.StatusCreated || held != 0 || len(fake.clawbacks) != 1 || balanceOf(t, ctx, org, "r13m") != 0 {
		t.Fatalf("FINDING: after a lost dispute the retry answered %d and the host holds %d (clawbacks %d)", r.StatusCode, held, len(fake.clawbacks))
	}
	deliver(t, ctx, secret, disputeEvent("evt_r13m_won", "dispute.state.updated", "dp_r13m", "sqpay_r13m", "WON", 4480000))
	deliver(t, ctx, secret, disputeEvent("evt_r13m_won2", "dispute.state.updated", "dp_r13m", "sqpay_r13m", "WON", 4480000))
	held, _ = fake.Balance(ctx, ledgerOrg(org), "r13m", "usd", org.TestMode())
	if held != 4000000 || balanceOf(t, ctx, org, "r13m") != 4000000 {
		t.Fatalf("the dispute won gave back host %d, commerce %d, want 4000000 once", held, balanceOf(t, ctx, org, "r13m"))
	}
}

// The plan's reload switches on only once the order's balance is credited: while it
// is owed, a sweep charges nothing.
func TestRed13_ReloadFiresWhileTheOrderBalanceIsOwed(t *testing.T) {
	fake := &flakyLedger{fakeLedger: newFakeLedger(), failures: 1}
	injectLedger(t, fake)
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("r13c")
	m := squareMock("cust_r13c", "ccof_r13c", "sqpay_r13c")
	withFakeSquare(t, m)
	row := orderRow(t, ctx, "r13c-plan", 20, 500)
	row.Limits.Reload = &plan.Reload{ThresholdCents: 1000, AmountCents: 5000}
	if err := row.Update(); err != nil {
		t.Fatal(err)
	}
	body := `{"sourceId":"cnon:ok","planId":"r13c-plan","quantity":20,"interval":"year","balanceCents":4000000}`
	key := map[string]string{"X-Idempotency-Key": "r13c"}
	if r := invokeSubscribeCard(org, ctx, body, key); r.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("order: %d %s", r.StatusCode, bodyText(r))
	}
	db := datastore.New(org.Namespaced(ctx))
	if rule := loadAutoRecharge(db, org.Name); rule != nil && rule.Enabled {
		t.Fatalf("FINDING: the reload is on while the order's balance is owed: %+v", rule)
	}
	m.chargeRef = "sqpay_r13c_reload"
	reloadPayer(ctx, nil, org, db, org.Name)
	m.chargeRef = "sqpay_r13c"
	if m.chargeCalls != 1 {
		t.Fatalf("FINDING: the plan's reload charged the card (%d charges) while the order's balance was owed", m.chargeCalls)
	}
	again := invokeSubscribeCard(org, ctx, body, key)
	if sale := receiptOf(t, again); again.StatusCode != http.StatusCreated || sale.Reload == nil {
		t.Fatalf("the completed order should switch the plan's reload on: %d %+v", again.StatusCode, sale)
	}
	if rule := loadAutoRecharge(db, org.Name); rule == nil || !rule.Enabled {
		t.Fatalf("the reload is off after the credit: %+v", rule)
	}
}

// One payment is one deposit: fund run twice for one payment — a retry completing
// a sale's credit — writes one deposit and answers it both times.
func TestFundCreditsOnePaymentOnce(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("fund-once")
	db := datastore.New(org.Namespaced(ctx))
	a, _, err := fund(ctx, nil, org, db, "fund-once", 40000, currency.USD, processor.Square, "sqpay_fund_once")
	if err != nil {
		t.Fatal(err)
	}
	b, _, err := fund(ctx, nil, org, db, "fund-once", 40000, currency.USD, processor.Square, "sqpay_fund_once")
	if err != nil {
		t.Fatal(err)
	}
	if a != b || depositsOf(t, db, "fund-once", "sqpay_fund_once") != 1 || balanceOf(t, ctx, org, "fund-once") != 40000 {
		t.Fatalf("FINDING: one payment funded twice: %q then %q, %d deposits, balance %d",
			a, b, depositsOf(t, db, "fund-once", "sqpay_fund_once"), balanceOf(t, ctx, org, "fund-once"))
	}
}

// plansDown is the store with its plan rows unreadable: every read of kind plan
// fails, as a lost connection does, and every other kind reads as it did.
type plansDown struct{ dbpkg.DB }

var errReadLost = errors.New("the read was lost")

func (d plansDown) Get(ctx context.Context, k dbpkg.Key, dst interface{}) error {
	if k != nil && k.Kind() == "plan" {
		return errReadLost
	}
	return d.DB.Get(ctx, k, dst)
}

func (d plansDown) Query(kind string) dbpkg.Query {
	if kind == "plan" {
		return downQuery{d.DB.Query(kind)}
	}
	return d.DB.Query(kind)
}

// downQuery is a query of the plan rows that fails when it runs.
type downQuery struct{ dbpkg.Query }

func (q downQuery) Filter(f string, v interface{}) dbpkg.Query {
	return downQuery{q.Query.Filter(f, v)}
}
func (q downQuery) FilterField(f, op string, v interface{}) dbpkg.Query {
	return downQuery{q.Query.FilterField(f, op, v)}
}
func (q downQuery) Order(f string) dbpkg.Query     { return downQuery{q.Query.Order(f)} }
func (q downQuery) OrderDesc(f string) dbpkg.Query { return downQuery{q.Query.OrderDesc(f)} }
func (q downQuery) Limit(n int) dbpkg.Query        { return downQuery{q.Query.Limit(n)} }
func (q downQuery) Offset(n int) dbpkg.Query       { return downQuery{q.Query.Offset(n)} }
func (q downQuery) Ancestor(k dbpkg.Key) dbpkg.Query {
	return downQuery{q.Query.Ancestor(k)}
}
func (q downQuery) GetAll(context.Context, interface{}) ([]dbpkg.Key, error) {
	return nil, errReadLost
}
func (q downQuery) First(context.Context, interface{}) (dbpkg.Key, error) { return nil, errReadLost }
func (q downQuery) Count(context.Context) (int, error)                    { return 0, errReadLost }
func (q downQuery) Keys(context.Context) ([]dbpkg.Key, error)             { return nil, errReadLost }
func (q downQuery) Run(ctx context.Context) dbpkg.Iterator                { return downIter{q.Query.Run(ctx)} }

// downIter is the iterator of a query that failed: its first result is the error.
type downIter struct{ dbpkg.Iterator }

func (downIter) Next(interface{}) (dbpkg.Key, error) { return nil, errReadLost }

// A plan row that cannot be read is not a plan that is not there: the sale answers
// 503 and charges nothing, rather than selling the embedded catalog's plan of the
// same name (which prices by quote what the row may sell per seat). A plan that is
// truly not there is still the catalog's to answer, or a 404.
func TestAPlanReadThatFailsIsUnavailable(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("plan-unread")
	m := squareMock("cust_pu", "ccof_pu", "sqpay_pu")
	withFakeSquare(t, m)
	up := datastore.New(ctx).DB()
	datastore.SetDefaultDB(plansDown{up})
	_, err := resolveSubscriptionPlan(datastore.New(ctx), "go")
	_, sold := subscribe(ctx, org, SubscribeIn{Subject: "plan-unread", SourceID: "cnon:ok", PlanID: "go"})
	datastore.SetDefaultDB(up)
	if !errors.Is(err, errPlanUnread) {
		t.Fatalf("FINDING: a failed read of plan go answered %v, want errPlanUnread", err)
	}
	if !IsSaleUnavailable(sold) || m.chargeCalls != 0 {
		t.Fatalf("FINDING: a sale whose plan could not be read answered %v with %d charges, want unavailable and none", sold, m.chargeCalls)
	}
	if _, err := resolveSubscriptionPlan(datastore.New(ctx), "no-such-plan"); err == nil || errors.Is(err, errPlanUnread) {
		t.Fatalf("an unknown plan answered %v, want a miss", err)
	}
	if p, err := resolveSubscriptionPlan(datastore.New(ctx), "go"); err != nil || p.Slug != "go" {
		t.Fatalf("with the store back, plan go answered %v %v", p, err)
	}
}

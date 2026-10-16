package billing

// Who may change what an org pays, and how much auto-reload may charge.
//
// The org's own account — its auto-reload, its cards, a charge on its card on
// file — is its admins' to change, and a platform SuperAdmin's or a service's. A
// plain member is refused with a sentence that says who to ask. A member billed
// from a wallet of their own (a person account in the shared signup org) changes
// that wallet's cards; auto-reload is the org's rule and stays its admins'.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/autorecharge"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/util/bit"
	"github.com/hanzoai/commerce/util/permission"
	"github.com/hanzoai/commerce/util/test/ae"
)

// The callers every gate here is asked about, as the identity edge mints them.
var (
	asMember  = map[string]string{"X-User-Id": "carol", "X-User-IsOrgAdmin": "false"}
	asAdmin   = map[string]string{"X-User-Id": "dana", "X-User-IsOrgAdmin": "true"}
	asService = map[string]string{} // + the Admin permission bit, below
)

// drive runs handler at pattern for org, as the caller headers name; service
// adds the Admin permission bit a verified service token arrives with.
func drive(t *testing.T, ctx ae.Context, org *organization.Organization, method, path, pattern, body string, headers map[string]string, service bool, handler zip.Handler) *http.Response {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return driveSeeded(func(c *zip.Ctx) {
		c.Locals("organization", org)
		if service {
			c.Locals("permissions", bit.Field(permission.Admin|permission.Live))
		}
		c.SetContext(ctx)
	}, pattern, req, handler)
}

func setRecharge(t *testing.T, ctx ae.Context, org *organization.Organization, body string, headers map[string]string, service bool) *http.Response {
	t.Helper()
	return drive(t, ctx, org, http.MethodPut, "/v1/billing/recharge", "/v1/billing/recharge", body, headers, service, SetAutoRecharge)
}

func storedRule(t *testing.T, ctx ae.Context, org *organization.Organization) *autorecharge.AutoRecharge {
	t.Helper()
	return loadAutoRecharge(datastore.New(org.Namespaced(ctx)), org.Name)
}

// A member cannot arm, change or switch off the org's auto-reload; its admin, a
// service and a platform caller can. The refusal names the org and who to ask.
func TestOnlyAnOrgAdminSetsAutoReload(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("reload-gate")
	seedSavedCard(t, datastore.New(org.Namespaced(ctx)), org.Name, "ccof_rg", "cust_rg")
	rule := `{"enabled":true,"thresholdCents":500,"amountCents":2500}`

	resp := setRecharge(t, ctx, org, rule, asMember, false)
	body := bodyOf(resp)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("a member set auto-reload: %d %s, want 403", resp.StatusCode, body)
	}
	if !strings.Contains(body, "Only an admin of reload-gate can change its auto-reload. Ask an admin to do it.") {
		t.Fatalf("the refusal %s does not say who to ask", body)
	}
	if storedRule(t, ctx, org) != nil {
		t.Fatal("a refused member wrote a rule")
	}

	// A member of the shared signup org, billed from their own wallet, still
	// cannot: the rule is the org's and charges the org's card.
	signup := moneyOrg("hanzo")
	if resp := setRecharge(t, ctx, signup, rule, map[string]string{"X-User-Id": "alice", "X-User-IsOrgAdmin": "false"}, false); resp.StatusCode != http.StatusForbidden {
		t.Fatalf("a signup-org member set the platform org's auto-reload: %d %s", resp.StatusCode, bodyOf(resp))
	}

	for name, who := range map[string]struct {
		headers map[string]string
		service bool
	}{
		"the org's admin": {asAdmin, false},
		"a service":       {asService, true},
		"a SuperAdmin":    {map[string]string{"X-User-Id": "z", "X-User-IsAdmin": "true"}, true},
	} {
		if resp := setRecharge(t, ctx, org, rule, who.headers, who.service); resp.StatusCode != http.StatusOK {
			t.Errorf("%s setting auto-reload: %d %s, want 200", name, resp.StatusCode, bodyOf(resp))
		}
	}
	if got := storedRule(t, ctx, org); got == nil || !got.Enabled || got.AmountCents != 2500 {
		t.Fatalf("the admin's rule was not stored: %+v", got)
	}
}

// The reload amount is a top-up's own range, $5 to $5,000, and the threshold is
// $0 to $5,000 — refused with the range in the sentence, and nothing stored.
func TestAutoReloadIsBounded(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("reload-bounds")
	seedSavedCard(t, datastore.New(org.Namespaced(ctx)), org.Name, "ccof_rb", "cust_rb")

	for _, tc := range []struct {
		amount, threshold int64
		want              int
		says              string
	}{
		{0, 500, 400, "The reload amount must be between $5 and $5,000."},
		{499, 500, 400, "The reload amount must be between $5 and $5,000."},
		{500001, 500, 400, "The reload amount must be between $5 and $5,000."},
		{100000000, 500, 400, "The reload amount must be between $5 and $5,000."},
		{2500, -1, 400, "The balance to reload below must be between $0 and $5,000."},
		{2500, 500001, 400, "The balance to reload below must be between $0 and $5,000."},
		{500, 0, 200, ""},
		{500000, 500000, 200, ""},
	} {
		body := fmt.Sprintf(`{"enabled":true,"thresholdCents":%d,"amountCents":%d}`, tc.threshold, tc.amount)
		resp := setRecharge(t, ctx, org, body, asAdmin, false)
		got := bodyOf(resp)
		if resp.StatusCode != tc.want || !strings.Contains(got, tc.says) {
			t.Errorf("amount %d threshold %d: %d %s, want %d saying %q", tc.amount, tc.threshold, resp.StatusCode, got, tc.want, tc.says)
		}
		if tc.want != 200 {
			if r := storedRule(t, ctx, org); r != nil && r.AmountCents == tc.amount && r.ThresholdCents == tc.threshold {
				t.Errorf("amount %d threshold %d was stored after its refusal", tc.amount, tc.threshold)
			}
		}
	}

	// Off is always allowed, whatever the old rule held, so a rule from before
	// the bound can still be switched off.
	cfg := storedRule(t, ctx, org)
	cfg.AmountCents = 100000000
	if err := cfg.Update(); err != nil {
		t.Fatal(err)
	}
	if resp := setRecharge(t, ctx, org, `{"enabled":false,"thresholdCents":0,"amountCents":100000000}`, asAdmin, false); resp.StatusCode != http.StatusOK {
		t.Fatalf("switching off an out-of-range rule: %d %s, want 200", resp.StatusCode, bodyOf(resp))
	}
}

// rechargeRun drives one sweep and decodes what it did.
func rechargeRun(t *testing.T, ctx ae.Context) RechargeRun {
	t.Helper()
	r := runRecharge(ctx)
	raw, _ := io.ReadAll(r.Body)
	if r.StatusCode != http.StatusOK {
		t.Fatalf("sweep: %d %s", r.StatusCode, raw)
	}
	var run RechargeRun
	if err := json.Unmarshal(raw, &run); err != nil {
		t.Fatalf("sweep body %s: %v", raw, err)
	}
	return run
}

// The sweep charges one org's card at most ReloadsPerDay times in a UTC day.
// Each reload here is a different amount, so each is a new charge rather than a
// replay of the last; the fourth waits for tomorrow and says so in its row.
func TestAutoReloadStopsAtItsDailyLimit(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	// The threshold stays above every post-charge balance, so each run is due.
	org := seedRechargeOrg(t, ctx, "reload-daily", 500, 1000000)
	m := squareMock("", "", "sqpay_daily")
	withFakeSquare(t, m)

	for i := 1; i <= ReloadsPerDay+1; i++ {
		cfg := storedRule(t, ctx, org)
		cfg.AmountCents = int64(500 + 100*i)
		if err := cfg.Update(); err != nil {
			t.Fatal(err)
		}
		run := rechargeRun(t, ctx)
		if i <= ReloadsPerDay {
			if m.chargeCalls != i || run.Charged != 1 {
				t.Fatalf("reload %d: %d charges, %d this run, want %d and 1", i, m.chargeCalls, run.Charged, i)
			}
			continue
		}
		if m.chargeCalls != ReloadsPerDay || run.Charged != 0 {
			t.Fatalf("reload %d charged the card: %d charges in a day, want %d", i, m.chargeCalls, ReloadsPerDay)
		}
		if len(run.Results) != 1 || !strings.Contains(run.Results[0].Error, "3 times today") {
			t.Fatalf("the skipped reload said %+v, want the daily limit named", run.Results)
		}
	}
	if cfg := storedRule(t, ctx, org); cfg.Reloads != ReloadsPerDay || cfg.Day != time.Now().UTC().Format(time.DateOnly) {
		t.Fatalf("the rule counted %d reloads on %q", cfg.Reloads, cfg.Day)
	}
}

// A new day starts the count again.
func TestAutoReloadCountsByTheDay(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := seedRechargeOrg(t, ctx, "reload-newday", 2500, 1000000)
	cfg := storedRule(t, ctx, org)
	cfg.Day = time.Now().UTC().AddDate(0, 0, -1).Format(time.DateOnly)
	cfg.Reloads = ReloadsPerDay
	if err := cfg.Update(); err != nil {
		t.Fatal(err)
	}
	m := squareMock("", "", "sqpay_newday")
	withFakeSquare(t, m)

	if run := rechargeRun(t, ctx); run.Charged != 1 || m.chargeCalls != 1 {
		t.Fatalf("yesterday's limit held today: charged %d, card calls %d", run.Charged, m.chargeCalls)
	}
	if cfg := storedRule(t, ctx, org); cfg.Reloads != 1 || cfg.Day != time.Now().UTC().Format(time.DateOnly) {
		t.Fatalf("today's count is %d on %q, want 1 today", cfg.Reloads, cfg.Day)
	}
}

// A re-fired sweep replays the reload it already made rather than charging
// again, and a replay is not a second reload against the limit.
func TestAutoReloadCountsAReplayOnce(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := seedRechargeOrg(t, ctx, "reload-replay", 2500, 1000000)
	m := squareMock("", "", "sqpay_replay")
	withFakeSquare(t, m)

	rechargeRun(t, ctx)
	rechargeRun(t, ctx)
	if m.chargeCalls != 1 {
		t.Fatalf("card calls %d, want 1", m.chargeCalls)
	}
	if cfg := storedRule(t, ctx, org); cfg.Reloads != 1 {
		t.Fatalf("one reload counted %d times", cfg.Reloads)
	}
}

// The org's cards — saving one, choosing the default, changing or removing one,
// charging one on file — are its admins'. A member is refused before anything is
// read or charged; the org's admin gets past the gate.
func TestOnlyAnOrgAdminChangesTheOrgsCards(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("cards-gate")
	pm := seedSavedCard(t, datastore.New(org.Namespaced(ctx)), org.Name, "ccof_cg", "cust_cg")
	m := squareMock("cust_cg", "ccof_cg", "sqpay_cg")
	withFakeSquare(t, m)

	for _, tc := range []struct {
		name, method, path, pattern, body, says string
		handler                                 zip.Handler
	}{
		{"save a card", http.MethodPost, "/v1/billing/methods", "/v1/billing/methods",
			`{"type":"card","providerRef":"cnon:card-nonce-ok","providerType":"square"}`, "change its cards", CreatePaymentMethod},
		{"change a card", http.MethodPatch, "/v1/billing/methods/" + pm.Id(), "/v1/billing/methods/:id",
			`{"metadata":{"note":"x"}}`, "change its cards", UpdatePaymentMethod},
		{"choose the default", http.MethodPost, "/v1/billing/customers/cards-gate/default-payment-method", "/v1/billing/customers/:id/default-payment-method",
			`{"paymentMethodId":"` + pm.Id() + `"}`, "change its default card", SetDefaultPaymentMethod},
		{"charge the card on file", http.MethodPost, "/v1/billing/topup", "/v1/billing/topup",
			`{"paymentMethodId":"` + pm.Id() + `","amountCents":2500}`, "charge its card on file", Topup},
		{"remove a card", http.MethodDelete, "/v1/billing/methods/" + pm.Id(), "/v1/billing/methods/:id",
			``, "remove its cards", DetachPaymentMethod},
	} {
		calls := m.chargeCalls
		member := drive(t, ctx, org, tc.method, tc.path, tc.pattern, tc.body, asMember, false, tc.handler)
		body := bodyOf(member)
		if member.StatusCode != http.StatusForbidden || !strings.Contains(body, "Only an admin of cards-gate can "+tc.says+". Ask an admin to do it.") {
			t.Errorf("%s as a member: %d %s, want 403 saying who to ask", tc.name, member.StatusCode, body)
		}
		if m.chargeCalls != calls {
			t.Fatalf("%s as a member reached the card", tc.name)
		}
		admin := drive(t, ctx, org, tc.method, tc.path, tc.pattern, tc.body, asAdmin, false, tc.handler)
		if admin.StatusCode == http.StatusForbidden {
			t.Errorf("%s as the org's admin: 403 %s — the gate refuses everyone", tc.name, bodyOf(admin))
		}
	}
}

// A member of the shared signup org pays from a wallet of their own, so its card
// on file is theirs to charge — the gate admits them, and the subject guard below
// it still keeps them to their own.
func TestAMemberChargesTheirOwnWalletsCard(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("hanzo")
	pm := seedSavedCard(t, datastore.New(org.Namespaced(ctx)), "hanzo/alice", "ccof_al", "cust_al")
	m := squareMock("cust_al", "ccof_al", "sqpay_al")
	withFakeSquare(t, m)
	alice := map[string]string{"X-User-Id": "alice", "X-User-IsOrgAdmin": "false"}

	resp := drive(t, ctx, org, http.MethodPost, "/v1/billing/topup", "/v1/billing/topup",
		`{"paymentMethodId":"`+pm.Id()+`","amountCents":2500}`, alice, false, Topup)
	if resp.StatusCode != http.StatusOK || m.chargeCalls != 1 {
		t.Fatalf("a member charging their own wallet's card: %d %s, %d charges, want 200 and 1", resp.StatusCode, bodyOf(resp), m.chargeCalls)
	}
	if got := balanceOf(t, ctx, org, "hanzo/alice"); got != 2500 {
		t.Fatalf("their wallet holds %d, want 2500", got)
	}
}

// An edited rule is the rule on file. A query-loaded row once carried no key, so
// its update wrote a second row the next read never found: the save answered with
// the new values and the old rule kept charging.
func TestAnEditedAutoReloadIsTheRuleOnFile(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("reload-edit")
	seedSavedCard(t, datastore.New(org.Namespaced(ctx)), org.Name, "ccof_re", "cust_re")

	for _, rule := range []string{
		`{"enabled":true,"thresholdCents":500,"amountCents":2500}`,
		`{"enabled":true,"thresholdCents":1000,"amountCents":5000}`,
		`{"enabled":false,"thresholdCents":1000,"amountCents":5000}`,
	} {
		if resp := setRecharge(t, ctx, org, rule, asAdmin, false); resp.StatusCode != http.StatusOK {
			t.Fatalf("set %s: %d %s", rule, resp.StatusCode, bodyOf(resp))
		}
	}
	got := readRecharge(t, org, ctx)
	if !strings.Contains(got, `"amountCents":5000`) || !strings.Contains(got, `"enabled":false`) || !strings.Contains(got, `"thresholdCents":1000`) {
		t.Fatalf("the rule on file is %s, want the last one written", got)
	}
	rows := make([]*autorecharge.AutoRecharge, 0)
	if _, err := autorecharge.Query(datastore.New(org.Namespaced(ctx))).Filter("UserId=", org.Name).GetAll(&rows); err != nil || len(rows) != 1 {
		t.Fatalf("%d rule rows for one org (%v), want one", len(rows), err)
	}
}

// midCharge charges like the mock and runs hook once, while the charge is in
// flight — the moment a real processor call is out and a person can still press
// a button, or another sweep can start.
type midCharge struct {
	*MockSquareProcessor
	hook func()
}

func (h *midCharge) Charge(ctx context.Context, req processor.PaymentRequest) (*processor.PaymentResult, error) {
	if f := h.hook; f != nil {
		h.hook = nil
		f()
	}
	return h.MockSquareProcessor.Charge(ctx, req)
}

func withMidCharge(t *testing.T, h *midCharge) {
	t.Helper()
	old := processorsForOrg
	processorsForOrg = func(*organization.Organization) *processor.Registry {
		reg := processor.NewRegistry(processor.DefaultConfig())
		reg.Register(h)
		return reg
	}
	t.Cleanup(func() { processorsForOrg = old })
}

// An admin who switches auto-reload off while a reload is in flight stays
// switched off: the sweep records its reload on the rule as it then stands
// rather than writing back the copy it read before charging.
func TestAnAdminsSwitchOffDuringAReloadHolds(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := seedRechargeOrg(t, ctx, "reload-race", 2500, 1000000)
	h := &midCharge{MockSquareProcessor: squareMock("", "", "sqpay_race")}
	h.hook = func() {
		if resp := setRecharge(t, ctx, org, `{"enabled":false,"thresholdCents":0,"amountCents":0}`, asAdmin, false); resp.StatusCode != http.StatusOK {
			t.Fatalf("switch-off during the reload: %d %s", resp.StatusCode, bodyOf(resp))
		}
	}
	withMidCharge(t, h)

	rechargeRun(t, ctx)
	cfg := storedRule(t, ctx, org)
	if cfg.Enabled {
		t.Fatalf("the sweep switched auto-reload back on after the admin switched it off: %+v", cfg)
	}
	if cfg.Reloads != 1 {
		t.Fatalf("the reload that was in flight counted %d, want 1", cfg.Reloads)
	}
}

// Two sweeps that overlap cannot both take the day's last reload: the count is
// reserved before the card is charged.
func TestOverlappingSweepsShareTheDailyLimit(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := seedRechargeOrg(t, ctx, "reload-overlap", 2500, 1000000)
	cfg := storedRule(t, ctx, org)
	cfg.Day, cfg.Reloads = time.Now().UTC().Format(time.DateOnly), ReloadsPerDay-1
	if err := cfg.Update(); err != nil {
		t.Fatal(err)
	}
	h := &midCharge{MockSquareProcessor: squareMock("", "", "sqpay_overlap")}
	var second RechargeRun
	h.hook = func() { second = rechargeRun(t, ctx) }
	withMidCharge(t, h)

	first := rechargeRun(t, ctx)
	if h.chargeCalls != 1 || first.Charged+second.Charged != 1 {
		t.Fatalf("two overlapping sweeps charged %d times (%d + %d) with one reload left today", h.chargeCalls, first.Charged, second.Charged)
	}
	if got := storedRule(t, ctx, org).Reloads; got != ReloadsPerDay {
		t.Fatalf("the day counts %d reloads, want %d", got, ReloadsPerDay)
	}
}

// A decline is an attempt on the card and counts against the day, so a failing
// card is tried at most ReloadsPerDay times; a charge refused before the
// processor was asked (here, an amount out of bounds) touched no card and does
// not count.
func TestADeclineCountsAndAnUnaskedChargeDoesNot(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := seedRechargeOrg(t, ctx, "reload-decline", 2500, 1000000)
	m := squareMock("", "", "sqpay_decline")
	m.chargeErr = errors.New("card declined")
	withFakeSquare(t, m)
	db := datastore.New(org.Namespaced(ctx))
	for i := 0; i < ReloadsPerDay+2; i++ {
		if _, _, err := changeRule(db, org.Name, func(c *autorecharge.AutoRecharge) bool { c.AmountCents = int64(2500 + i); return true }); err != nil {
			t.Fatal(err)
		}
		if run := rechargeRun(t, ctx); run.Charged != 0 {
			t.Fatalf("a declined reload was reported charged: %+v", run)
		}
	}
	if m.chargeCalls != ReloadsPerDay {
		t.Fatalf("a declining card was tried %d times in a day, want %d", m.chargeCalls, ReloadsPerDay)
	}

	other := seedRechargeOrg(t, ctx, "reload-unasked", 2500, 1000000)
	odb := datastore.New(other.Namespaced(ctx))
	if _, _, err := changeRule(odb, other.Name, func(c *autorecharge.AutoRecharge) bool { c.AmountCents = 100000000; return true }); err != nil {
		t.Fatal(err)
	}
	rechargeRun(t, ctx)
	if got := storedRule(t, ctx, other).Reloads; got != 0 {
		t.Fatalf("a charge the processor never saw counted %d against the day", got)
	}
}

// Before the key was bound, every save of an existing rule went to a second,
// parentless row the reader never saw: a customer who switched auto-reload off
// was still charged. The first read now honours that switch-off and removes the
// row.
func TestAnEditThatWentAstrayIsAdopted(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := seedRechargeOrg(t, ctx, "reload-astray", 2500, 1000000)
	db := datastore.New(org.Namespaced(ctx))

	// The old save path, exactly: query-loaded, rebound with no key, updated.
	rows := make([]*autorecharge.AutoRecharge, 0)
	if _, err := autorecharge.Query(db).Ancestor(db.NewKey("synckey", "", 1, nil)).Filter("UserId=", org.Name).GetAll(&rows); err != nil || len(rows) != 1 {
		t.Fatalf("seed rows: %d %v", len(rows), err)
	}
	rows[0].Rebind(db)
	rows[0].Enabled = false
	if err := rows[0].Update(); err != nil {
		t.Fatal(err)
	}
	all := make([]*autorecharge.AutoRecharge, 0)
	if _, err := autorecharge.Query(db).Filter("UserId=", org.Name).GetAll(&all); err != nil || len(all) != 2 {
		t.Fatalf("the old save left %d rows (%v), want the rule and its stray", len(all), err)
	}

	m := squareMock("", "", "sqpay_astray")
	withFakeSquare(t, m)
	rechargeRun(t, ctx)
	if m.chargeCalls != 0 {
		t.Fatalf("a rule its customer switched off was charged %d times", m.chargeCalls)
	}
	if cfg := storedRule(t, ctx, org); cfg.Enabled {
		t.Fatalf("the rule on file is still armed: %+v", cfg)
	}
	all = all[:0]
	if _, err := autorecharge.Query(db).Filter("UserId=", org.Name).GetAll(&all); err != nil || len(all) != 1 {
		t.Fatalf("%d rows remain (%v), want the rule alone", len(all), err)
	}
}

// A stray that would ARM a rule the customer saw as off is not adopted: every
// read showed the rule on file, and an off-session charge needs consent somebody
// saw recorded.
func TestAStrayNeverArmsARule(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := seedRechargeOrg(t, ctx, "reload-arm", 2500, 1000000)
	db := datastore.New(org.Namespaced(ctx))
	on := storedRule(t, ctx, org)
	on.Enabled = false
	if err := on.Update(); err != nil {
		t.Fatal(err)
	}
	rows := make([]*autorecharge.AutoRecharge, 0)
	if _, err := autorecharge.Query(db).Ancestor(db.NewKey("synckey", "", 1, nil)).Filter("UserId=", org.Name).GetAll(&rows); err != nil || len(rows) != 1 {
		t.Fatalf("seed rows: %d %v", len(rows), err)
	}
	rows[0].Rebind(db)
	rows[0].Enabled = true
	rows[0].AmountCents = 500000
	if err := rows[0].Update(); err != nil {
		t.Fatal(err)
	}

	m := squareMock("", "", "sqpay_arm")
	withFakeSquare(t, m)
	rechargeRun(t, ctx)
	if m.chargeCalls != 0 {
		t.Fatalf("a stray armed a rule the customer saw as off and it charged %d times", m.chargeCalls)
	}
	if cfg := storedRule(t, ctx, org); cfg.Enabled || cfg.AmountCents != 2500 {
		t.Fatalf("the rule on file moved: %+v", cfg)
	}
}

// A stray the old sweep wrote — the rule as it read it, stamped — adds nothing,
// and the rule on file stands.
func TestAStrayThatMatchesChangesNothing(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := seedRechargeOrg(t, ctx, "reload-stamp", 2500, 1000000)
	db := datastore.New(org.Namespaced(ctx))
	rows := make([]*autorecharge.AutoRecharge, 0)
	if _, err := autorecharge.Query(db).Ancestor(db.NewKey("synckey", "", 1, nil)).Filter("UserId=", org.Name).GetAll(&rows); err != nil || len(rows) != 1 {
		t.Fatalf("seed rows: %d %v", len(rows), err)
	}
	rows[0].Rebind(db)
	rows[0].LastRechargedAt = "2026-10-01T00:00:00Z"
	if err := rows[0].Update(); err != nil {
		t.Fatal(err)
	}
	if cfg := storedRule(t, ctx, org); !cfg.Enabled || cfg.AmountCents != 2500 {
		t.Fatalf("a matching stray moved the rule: %+v", cfg)
	}
	all := make([]*autorecharge.AutoRecharge, 0)
	if _, err := autorecharge.Query(db).Filter("UserId=", org.Name).GetAll(&all); err != nil || len(all) != 1 {
		t.Fatalf("%d rows remain (%v), want the rule alone", len(all), err)
	}
}

// Only a person account of the caller's own is theirs: a credential billed from
// another account in the org — a project's — is the org's admins' to change.
func TestAProjectWalletIsNotTheCallersOwn(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("cards-project")
	pm := seedSavedCard(t, datastore.New(org.Namespaced(ctx)), "cards-project/site", "ccof_pj", "cust_pj")
	proj := map[string]string{"X-User-Id": "carol", "X-User-IsOrgAdmin": "false", "X-Billing-Account-Id": "project:cards-project/site"}
	resp := drive(t, ctx, org, http.MethodPost, "/v1/billing/topup", "/v1/billing/topup",
		`{"paymentMethodId":"`+pm.Id()+`","amountCents":2500}`, proj, false, Topup)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("a project-billed credential charged the project's card: %d %s, want 403", resp.StatusCode, bodyOf(resp))
	}
}

// unanswered settles the charge at the processor and loses the answer — a
// timeout or a reset after the money moved.
type unanswered struct{ *MockSquareProcessor }

func (u *unanswered) Charge(ctx context.Context, req processor.PaymentRequest) (*processor.PaymentResult, error) {
	_, _ = u.MockSquareProcessor.Charge(ctx, req)
	return nil, context.DeadlineExceeded
}

// A reload whose answer was lost may have moved money, so it keeps its place
// against the daily limit: the processor never settles more than ReloadsPerDay
// reloads in a day however many answers go missing. Each sweep here moves the
// amount a cent, as a new idempotency window would, so each is a new charge.
func TestALostAnswerCountsAgainstTheDay(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := seedRechargeOrg(t, ctx, "reload-lost", 2500, 1000000)
	m := squareMock("", "", "sqpay_lost")
	u := &unanswered{m}
	old := processorsForOrg
	processorsForOrg = func(*organization.Organization) *processor.Registry {
		reg := processor.NewRegistry(processor.DefaultConfig())
		reg.Register(u)
		return reg
	}
	t.Cleanup(func() { processorsForOrg = old })

	db := datastore.New(org.Namespaced(ctx))
	for i := 0; i < ReloadsPerDay+3; i++ {
		if _, _, err := changeRule(db, org.Name, func(c *autorecharge.AutoRecharge) bool { c.AmountCents = int64(2500 + i); return true }); err != nil {
			t.Fatal(err)
		}
		rechargeRun(t, ctx)
	}
	if m.chargeCalls != ReloadsPerDay {
		t.Fatalf("the processor settled %d reloads in a day, want %d", m.chargeCalls, ReloadsPerDay)
	}
}

package promo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/auth"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/billingevent"
	"github.com/hanzoai/commerce/models/coupon"
	"github.com/hanzoai/commerce/models/couponredemption"
	"github.com/hanzoai/commerce/models/plan"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/bit"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/commerce/util/permission"
	"github.com/hanzoai/commerce/util/test/ae"
)

// warmCoupons opens the platform namespace's coupon kinds once, so the writes a
// test drives and the reads it checks share one handle, and puts the plans a
// coupon may name into the plan authority: three paid plans, a free one and a
// contact-sales one.
func warmCoupons(ctx context.Context) {
	db := platform(ctx)
	_, _ = coupon.Query(db).Count()
	_, _ = couponredemption.Query(db).Count()
	_, _ = billingevent.Query(db).Count()
	adb := plan.AuthorityDB(ctx)
	if n, _ := plan.Query(adb).Count(); n > 0 {
		return
	}
	for _, p := range []struct {
		slug  string
		price int64
		sales bool
	}{{"dev", 2000, false}, {"max-5x", 10000, false}, {"max-20x", 20000, false}, {"free", 0, false}, {"enterprise", 0, true}} {
		row := plan.New(adb)
		row.Slug, row.Name, row.Price, row.ContactSales = p.slug, p.slug, currency.Cents(p.price), p.sales
		if err := row.Create(); err != nil {
			panic(err)
		}
	}
}

func at(t time.Time) *time.Time { return &t }

// setCoupon stores a plan coupon through the one write path, failing the test
// on refusal.
func setCoupon(t *testing.T, ctx context.Context, code string, spec CouponSpec) *CouponView {
	t.Helper()
	v, err := SetCoupon(ctx, code, spec, "z@hanzo.ai")
	if err != nil {
		t.Fatalf("SetCoupon(%s): %v", code, err)
	}
	return v
}

func TestCouponCode(t *testing.T) {
	cases := map[string]string{
		"50off":     "50OFF",
		"  50OFF  ": "50OFF",
		"Spring_26": "SPRING_26",
		"a-b":       "A-B",
		"":          "",
		"   ":       "",
		"50 OFF":    "",
		"50%":       "",
		"ÉTÉ":       "",
		"x'; drop":  "",
	}
	long := make([]byte, maxCouponCode+1)
	for i := range long {
		long[i] = 'A'
	}
	cases[string(long)] = ""
	cases[string(long[:maxCouponCode])] = string(long[:maxCouponCode])
	for in, want := range cases {
		if got := CouponCode(in); got != want {
			t.Errorf("CouponCode(%q) = %q, want %q", in, got, want)
		}
	}
}

// FindCoupon judges a stored coupon by one rule, and a coupon that is off, not
// yet open, or not a plan coupon reads exactly like one that does not exist.
func TestFindCoupon(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	warmCoupons(ctx)

	now := time.Now().UTC()
	setCoupon(t, ctx, "LIVE", CouponSpec{Percent: 50, Start: at(now.Add(-time.Hour)), End: at(now.Add(24 * time.Hour)), Limit: 5, Plans: []string{"dev", " MAX-5X ", "dev"}, Enabled: true})
	setCoupon(t, ctx, "OPEN", CouponSpec{Percent: 10, Plans: []string{"max-20x"}, Enabled: true})
	setCoupon(t, ctx, "OLD", CouponSpec{Percent: 50, Start: at(now.Add(-48 * time.Hour)), End: at(now.Add(-time.Hour)), Plans: []string{"dev"}, Enabled: true})
	setCoupon(t, ctx, "SOON", CouponSpec{Percent: 50, Start: at(now.Add(time.Hour)), End: at(now.Add(48 * time.Hour)), Plans: []string{"dev"}, Enabled: true})
	setCoupon(t, ctx, "OFF", CouponSpec{Percent: 50, Plans: []string{"dev"}, Enabled: false})
	used := setCoupon(t, ctx, "GONE", CouponSpec{Percent: 50, Limit: 1, Plans: []string{"dev"}, Enabled: true})
	if _, err := Reserve(ctx, used.Code, "org-gone", "fp:gone", now); err != nil {
		t.Fatalf("use GONE: %v", err)
	}

	// A cart coupon in the same namespace is never honoured as a plan coupon.
	db := platform(ctx)
	cart := coupon.New(db)
	cart.Code_ = "CART"
	cart.Type = coupon.Percent
	cart.Amount = 90
	cart.Enabled = true
	if err := cart.Create(); err != nil {
		t.Fatalf("seed cart coupon: %v", err)
	}
	// Two rows under one code answer neither.
	for i := 0; i < 2; i++ {
		d := coupon.New(db)
		d.Code_ = "TWICE"
		d.Type = coupon.Percent
		d.Filter = coupon.PlanFilter
		d.Amount = 50 + i*40
		d.Enabled = true
		if err := d.Create(); err != nil {
			t.Fatalf("seed duplicate coupon: %v", err)
		}
	}

	cases := []struct {
		code, reason string
		percent      int
	}{
		{"live", "", 50},
		{"  Live ", "", 50},
		{"OPEN", "", 10},
		{"OLD", CouponExpired, 0},
		{"SOON", CouponUnknown, 0},
		{"OFF", CouponUnknown, 0},
		{"GONE", CouponExhausted, 0},
		{"NOPE", CouponUnknown, 0},
		{"CART", CouponUnknown, 0},
		{"TWICE", CouponUnknown, 0},
		{"", CouponUnknown, 0},
		{"no spaces", CouponUnknown, 0},
	}
	for _, tc := range cases {
		o, reason := FindCoupon(ctx, tc.code, now)
		if reason != tc.reason {
			t.Errorf("FindCoupon(%q) reason = %q, want %q", tc.code, reason, tc.reason)
			continue
		}
		if tc.reason == "" && (o == nil || o.Percent != tc.percent) {
			t.Errorf("FindCoupon(%q) = %+v, want %d%%", tc.code, o, tc.percent)
		}
	}

	live, _ := FindCoupon(ctx, "LIVE", now)
	for slug, want := range map[string]bool{"dev": true, "max-5x": true, "max-20x": false, "pro": false} {
		if got := live.AppliesTo(slug); got != want {
			t.Errorf("LIVE.AppliesTo(%q) = %v, want %v", slug, got, want)
		}
	}
	if live.End == nil || live.Label() != "LIVE — 50% off first month" {
		t.Errorf("LIVE end=%v label=%q", live.End, live.Label())
	}
	open, _ := FindCoupon(ctx, "OPEN", now)
	if !open.AppliesTo("max-20x") || open.AppliesTo("dev") || open.End != nil {
		t.Errorf("OPEN covers max-20x alone with no end; got end=%v", open.End)
	}

	// A plan coupon written with no plan list covers nothing, never everything.
	bare := coupon.New(db)
	bare.Code_ = "BARE"
	bare.Type = coupon.Percent
	bare.Filter = coupon.PlanFilter
	bare.Amount = 50
	bare.Enabled = true
	if err := bare.Create(); err != nil {
		t.Fatalf("seed bare coupon: %v", err)
	}
	if o, reason := FindCoupon(ctx, "BARE", now); reason != "" || o.AppliesTo("dev") || o.AppliesTo("max-20x") {
		t.Errorf("a coupon naming no plan applies somewhere (reason %q)", reason)
	}
}

// A coupon names the plans it covers, and each must be a paid plan on sale: an
// empty list, a typo, a free plan and a contact-sales plan are refused at the PUT.
func TestSetCoupon_PlansMustBePaidPlansOnSale(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	warmCoupons(ctx)

	for _, plans := range [][]string{nil, {}, {" "}, {"pro"}, {"max"}, {"dev", "pro"}, {"free"}, {"enterprise"}} {
		if _, err := SetCoupon(ctx, "PICKY", CouponSpec{Percent: 50, Plans: plans, Enabled: true}, "z"); !IsCouponSpecRefused(err) {
			t.Errorf("plans %q = %v, want a spec refusal", plans, err)
		}
	}
	if _, err := ReadCoupon(ctx, "PICKY"); !errors.Is(err, ErrCouponNotFound) {
		t.Fatalf("a refused coupon was stored: %v", err)
	}
	v := setCoupon(t, ctx, "PICKY", CouponSpec{Percent: 50, Plans: []string{" DEV ", "max-5x", "dev"}, Enabled: true})
	if len(v.Plans) != 2 || v.Plans[0] != "dev" || v.Plans[1] != "max-5x" {
		t.Fatalf("plans stored as %v, want [dev max-5x]", v.Plans)
	}
}

// A coupon switched off stays off when read back. The model defaults Enabled to
// true and a query-bound load re-applies defaults, so this pins the load path.
func TestSetCoupon_DisabledStaysDisabled(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	warmCoupons(ctx)

	setCoupon(t, ctx, "FLIP", CouponSpec{Percent: 50, Plans: []string{"dev"}, Enabled: true})
	setCoupon(t, ctx, "FLIP", CouponSpec{Percent: 50, Plans: []string{"dev"}, Enabled: false})
	v, err := ReadCoupon(ctx, "flip")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if v.Enabled {
		t.Fatal("a disabled coupon read back enabled")
	}
	if _, reason := FindCoupon(ctx, "FLIP", time.Now()); reason != CouponUnknown {
		t.Fatalf("disabled coupon reason = %q, want unknown", reason)
	}
	if _, err := Reserve(ctx, "FLIP", "acme", "fp:1", time.Now()); !errors.As(err, new(CouponRefusal)) {
		t.Fatalf("reserve on a disabled coupon = %v, want a refusal", err)
	}
}

// One use per org and one per card; a released use is given back whole.
func TestReserve_OncePerOrgAndCard(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	warmCoupons(ctx)
	now := time.Now()

	setCoupon(t, ctx, "ONCE", CouponSpec{Percent: 50, Limit: 10, Plans: []string{"dev"}, Enabled: true})

	h, err := Reserve(ctx, "once", "acme", "fp:a", now)
	if err != nil {
		t.Fatalf("first use: %v", err)
	}
	if _, err := Reserve(ctx, "ONCE", "ACME", "fp:b", now); !errors.Is(err, ErrCouponRedeemedByOrg) {
		t.Fatalf("same org, other card = %v, want ErrCouponRedeemedByOrg", err)
	}
	if _, err := Reserve(ctx, "ONCE", "other", "fp:a", now); !errors.Is(err, ErrCouponRedeemedByCard) {
		t.Fatalf("other org, same card = %v, want ErrCouponRedeemedByCard", err)
	}
	if !Redeemed(ctx, "once", "acme") || Redeemed(ctx, "once", "other") {
		t.Fatal("Redeemed disagrees with the reservation")
	}
	if v, _ := ReadCoupon(ctx, "ONCE"); v.Used != 1 {
		t.Fatalf("used = %d after one use, want 1", v.Used)
	}

	h.Release()
	h.Release() // the second release is a no-op
	if v, _ := ReadCoupon(ctx, "ONCE"); v.Used != 0 {
		t.Fatalf("used = %d after release, want 0", v.Used)
	}
	if Redeemed(ctx, "ONCE", "acme") {
		t.Fatal("a released use still marks the org")
	}
	if _, err := Reserve(ctx, "ONCE", "acme", "fp:a", now); err != nil {
		t.Fatalf("the org and card may redeem again after a release: %v", err)
	}
	if _, err := Reserve(ctx, "ONCE", "acme2", "", now); err == nil {
		t.Fatal("a use with no card was reserved")
	}
}

// Concurrent buyers never take more uses than the limit.
func TestReserve_ConcurrentNeverExceedsLimit(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	warmCoupons(ctx)

	const limit, buyers = 5, 40
	setCoupon(t, ctx, "RACE", CouponSpec{Percent: 50, Limit: limit, Plans: []string{"dev"}, Enabled: true})

	var wg sync.WaitGroup
	var mu sync.Mutex
	won, exhausted := 0, 0
	start := make(chan struct{})
	for i := 0; i < buyers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_, err := Reserve(ctx, "RACE", fmt.Sprintf("org-%d", i), fmt.Sprintf("fp:%d", i), time.Now())
			mu.Lock()
			defer mu.Unlock()
			var cr CouponRefusal
			switch {
			case err == nil:
				won++
			case errors.As(err, &cr) && cr.Reason == CouponExhausted:
				exhausted++
			default:
				t.Errorf("buyer %d: %v", i, err)
			}
		}(i)
	}
	close(start)
	wg.Wait()

	if won != limit || exhausted != buyers-limit {
		t.Fatalf("won=%d exhausted=%d, want %d and %d", won, exhausted, limit, buyers-limit)
	}
	v, _ := ReadCoupon(ctx, "RACE")
	if v.Used != limit {
		t.Fatalf("used = %d, want %d", v.Used, limit)
	}
	n, _ := couponredemption.Query(platform(ctx)).Filter("CouponCode=", "RACE").Count()
	if n != 2*limit {
		t.Fatalf("redemption rows = %d, want %d (one per org and one per card)", n, 2*limit)
	}
}

func couponApp(super bool) *zip.App {
	a := zip.New(zip.Config{DisableStartupMessage: true})
	a.Use(zip.H(func(c *zip.Ctx) error {
		if super {
			c.Locals("iam_authenticated", true)
			c.Locals("permissions", bit.Field(permission.Admin|permission.Live))
			c.Locals("iam_claims", &auth.IAMClaims{Owner: "admin", Email: "z@hanzo.ai"})
		} else {
			c.Locals("iam_authenticated", true)
			c.Locals("permissions", bit.Field(permission.Admin|permission.Live))
			c.Locals("iam_claims", &auth.IAMClaims{Owner: "acme", IsAdmin: true})
		}
		return c.Next()
	}))
	a.Raw(http.MethodGet, "/v1/platform/coupons/:code", GetCoupon)
	a.Raw(http.MethodPut, "/v1/platform/coupons/:code", PutCoupon)
	return a
}

func couponDo(t *testing.T, a *zip.App, method, code, body string) (int, string) {
	t.Helper()
	var r *http.Request
	if body != "" {
		r = httptest.NewRequest(method, "/v1/platform/coupons/"+code, bytes.NewReader([]byte(body)))
		r.Header.Set("Content-Type", "application/json")
	} else {
		r = httptest.NewRequest(method, "/v1/platform/coupons/"+code, nil)
	}
	resp, err := a.Test(r)
	if err != nil {
		t.Fatalf("%s coupon: %v", method, err)
	}
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(raw)
}

// The SuperAdmin surface: PUT creates and replaces, GET reads back, both are
// SuperAdmin only, a bad spec is the caller's 400, and every change is audited
// with who made it.
func TestCouponAdmin(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	warmCoupons(context.Background())
	warmPlatformNS()

	start := time.Now().UTC().Truncate(time.Second)
	end := start.Add(24 * time.Hour)
	body := fmt.Sprintf(`{"percent":50,"start":%q,"end":%q,"limit":5,"plans":["dev","max-5x","max-20x"],"enabled":true}`,
		start.Format(time.RFC3339), end.Format(time.RFC3339))

	if code, _ := couponDo(t, couponApp(false), http.MethodPut, "50OFF", body); code != 403 {
		t.Fatalf("org admin PUT = %d, want 403", code)
	}
	if code, _ := couponDo(t, couponApp(false), http.MethodGet, "50OFF", ""); code != 403 {
		t.Fatalf("org admin GET = %d, want 403", code)
	}

	a := couponApp(true)
	code, raw := couponDo(t, a, http.MethodPut, "50off", body)
	if code != 200 {
		t.Fatalf("PUT = %d %s", code, raw)
	}
	var v CouponView
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatalf("decode: %v (%s)", err, raw)
	}
	if v.Code != "50OFF" || v.Percent != 50 || v.Limit != 5 || !v.Enabled || v.Used != 0 ||
		len(v.Plans) != 3 || v.Start == nil || !v.Start.Equal(start) || v.End == nil || !v.End.Equal(end) {
		t.Fatalf("PUT answered %+v", v)
	}

	// A use, then an edit: the edit never resets Used.
	if _, err := Reserve(context.Background(), "50OFF", "acme", "fp:1", time.Now()); err != nil {
		t.Fatalf("use: %v", err)
	}
	if code, raw = couponDo(t, a, http.MethodPut, "50OFF", `{"percent":40,"limit":5,"plans":["dev"],"enabled":true}`); code != 200 {
		t.Fatalf("second PUT = %d %s", code, raw)
	}
	code, raw = couponDo(t, a, http.MethodGet, "50OFF", "")
	if code != 200 {
		t.Fatalf("GET = %d %s", code, raw)
	}
	v = CouponView{}
	_ = json.Unmarshal([]byte(raw), &v)
	if v.Percent != 40 || v.Used != 1 || v.Start != nil || v.End != nil || len(v.Plans) != 1 {
		t.Fatalf("after edit %+v, want 40%%, used 1, open window, one plan", v)
	}
	if code, _ := couponDo(t, a, http.MethodGet, "NONE", ""); code != 404 {
		t.Fatalf("GET of an unknown code = %d, want 404", code)
	}

	for _, bad := range []string{
		`{"percent":0,"enabled":true}`,
		`{"percent":101,"enabled":true}`,
		`{"percent":50,"limit":-1,"enabled":true}`,
		fmt.Sprintf(`{"percent":50,"start":%q,"end":%q}`, end.Format(time.RFC3339), start.Format(time.RFC3339)),
		`not json`,
	} {
		if code, raw := couponDo(t, a, http.MethodPut, "BAD", bad); code != 400 {
			t.Errorf("PUT %s = %d %s, want 400", bad, code, raw)
		}
	}
	if code, _ := couponDo(t, a, http.MethodPut, "no%20spaces", `{"percent":50}`); code != 400 {
		t.Errorf("PUT of a malformed code = %d, want 400", code)
	}

	// The audit trail: one created, one updated, each naming who.
	evts := make([]*billingevent.BillingEvent, 0)
	if _, err := billingevent.Query(platform(context.Background())).Filter("ObjectId=", "50OFF").GetAll(&evts); err != nil {
		t.Fatalf("audit query: %v", err)
	}
	types := map[string]int{}
	for _, e := range evts {
		types[e.Type]++
		if e.Data["by"] != "z@hanzo.ai" {
			t.Errorf("audit %s by = %v, want z@hanzo.ai", e.Type, e.Data["by"])
		}
	}
	if types["coupon.created"] != 1 || types["coupon.updated"] != 1 {
		t.Fatalf("audit events = %v, want one created and one updated", types)
	}
}

// A code already held by a cart coupon is not taken over by a plan coupon PUT.
func TestSetCoupon_RefusesNonPlanCode(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	warmCoupons(ctx)

	cart := coupon.New(datastore.New(nscontext.WithNamespace(ctx, platformNS)))
	cart.Code_ = "STORE"
	cart.Type = coupon.Flat
	cart.Amount = 500
	if err := cart.Create(); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := SetCoupon(ctx, "STORE", CouponSpec{Percent: 50, Plans: []string{"dev"}, Enabled: true}, "z"); !IsCouponSpecRefused(err) {
		t.Fatalf("SetCoupon over a cart coupon = %v, want a spec refusal", err)
	}
}

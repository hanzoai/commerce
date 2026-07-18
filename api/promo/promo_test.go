package promo

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	promotionModel "github.com/hanzoai/commerce/models/promotion"
	"github.com/hanzoai/commerce/util/bit"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/commerce/util/permission"
	"github.com/hanzoai/commerce/util/test/ae"
)

// warmPlatformNS opens the reserved "admin" namespace once so the Fiber-driven
// create and read share visibility (ae gives each namespace its own SQLite handle).
func warmPlatformNS() {
	db := datastore.New(nscontext.WithNamespace(context.Background(), platformNS))
	_, _ = promotionModel.Query(db).Count()
}

// superApp mounts the promo routes behind a seed that grants the platform-admin
// permission bit with NO IAM subject — the service-token principal MayReadPlatform
// admits. notSuper omits the bit so RequirePlatformAdmin refuses (403).
func app(super bool) *zip.App {
	a := zip.New(zip.Config{DisableStartupMessage: true})
	a.Use(func(c *zip.Ctx) error {
		if super {
			c.Locals("permissions", bit.Field(permission.Admin|permission.Live))
		}
		return c.Next()
	})
	a.Get("/v1/platform/promo", GetPromo)
	a.Put("/v1/platform/promo", PutPromo)
	return a
}

func do(t *testing.T, a *zip.App, method, body string) (*http.Response, string) {
	t.Helper()
	var r *http.Request
	if body != "" {
		r = httptest.NewRequest(method, "/v1/platform/promo", bytes.NewReader([]byte(body)))
		r.Header.Set("Content-Type", "application/json")
	} else {
		r = httptest.NewRequest(method, "/v1/platform/promo", nil)
	}
	resp, err := a.Fiber().Test(r)
	if err != nil {
		t.Fatalf("%s promo: %v", method, err)
	}
	raw, _ := io.ReadAll(resp.Body)
	return resp, string(raw)
}

// PUT then GET round-trips the canonical {percentOff,start,end,plans,active} shape,
// composed on ONE Promotion in the reserved platform namespace.
func TestPromo_PutGet_RoundTrip(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()
	warmPlatformNS()

	a := app(true)
	resp, _ := do(t, a, http.MethodPut,
		`{"percentOff":50,"plans":["pro","max"],"active":true,"end":"2099-01-01T00:00:00Z"}`)
	if resp.StatusCode != 200 {
		t.Fatalf("PUT status = %d", resp.StatusCode)
	}

	_, body := do(t, a, http.MethodGet, "")
	var got Promo
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("decode: %v (%s)", err, body)
	}
	if got.PercentOff != 50 || !got.Active {
		t.Fatalf("got %+v, want percentOff 50 active", got)
	}
	if len(got.Plans) != 2 || got.Plans[0] != "pro" || got.Plans[1] != "max" {
		t.Fatalf("plans = %v, want [pro max]", got.Plans)
	}
	if got.End == nil {
		t.Fatalf("end not persisted")
	}

	// Editing the singleton must not create a second promo — PUT again, still one.
	do(t, a, http.MethodPut, `{"percentOff":25,"active":false}`)
	db := datastore.New(nscontext.WithNamespace(context.Background(), platformNS))
	n, _ := promotionModel.Query(db).Filter("Code=", promoCode).Count()
	if n != 1 {
		t.Fatalf("promo rows = %d, want 1 (singleton)", n)
	}
}

// A non-SuperAdmin (no platform bit, no admin org) is refused — a customer must not
// configure the platform promo.
func TestPromo_Put_RefusesNonSuperAdmin(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()
	warmPlatformNS()

	resp, _ := do(t, app(false), http.MethodPut, `{"percentOff":90,"active":true}`)
	if resp.StatusCode != 403 {
		t.Fatalf("non-super PUT status = %d, want 403", resp.StatusCode)
	}
}

// Active() gates on status AND the UTC window: a drafted, expired, or not-yet-started
// promo never discounts; an active in-window one does.
func TestPromo_Active_WindowGate(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()
	warmPlatformNS()

	a := app(true)
	c := a.TestCtx(http.MethodGet, "/x") // any ctx to carry the namespace resolution.

	// none configured → nil.
	if p := Active(c); p != nil {
		t.Fatalf("no promo → Active = %+v, want nil", p)
	}

	// active, no window → live.
	do(t, a, http.MethodPut, `{"percentOff":50,"active":true}`)
	if p := Active(c); p == nil || p.PercentOff != 50 {
		t.Fatalf("active promo → Active = %+v, want live 50", p)
	}

	// drafted → nil.
	do(t, a, http.MethodPut, `{"percentOff":50,"active":false}`)
	if p := Active(c); p != nil {
		t.Fatalf("drafted promo → Active = %+v, want nil", p)
	}

	// expired window → nil.
	do(t, a, http.MethodPut, `{"percentOff":50,"active":true,"end":"2000-01-01T00:00:00Z"}`)
	if p := Active(c); p != nil {
		t.Fatalf("expired promo → Active = %+v, want nil", p)
	}

	// future start → nil.
	future := time.Now().UTC().Add(48 * time.Hour).Format(time.RFC3339)
	do(t, a, http.MethodPut, `{"percentOff":50,"active":true,"start":"`+future+`"}`)
	if p := Active(c); p != nil {
		t.Fatalf("not-yet-started promo → Active = %+v, want nil", p)
	}
}

// AppliesTo: empty Plans = all; a listed set = only those slugs.
func TestPromo_AppliesTo(t *testing.T) {
	all := &Promo{}
	if !all.AppliesTo("pro") || !all.AppliesTo("anything") {
		t.Fatal("empty plans must apply to all")
	}
	some := &Promo{Plans: []string{"pro", "max"}}
	if !some.AppliesTo("pro") || some.AppliesTo("developer") {
		t.Fatal("listed plans must apply only to listed slugs")
	}
}

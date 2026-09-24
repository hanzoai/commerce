package billing

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/creditgrant"
	"github.com/hanzoai/commerce/models/meter"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/pricingrule"
	"github.com/hanzoai/commerce/util/test/ae"
)

// previewOrg holds 3000c of metered usage for subject and a 1000c credit grant.
func previewOrg(t *testing.T, ctx context.Context, name, subject string) (*organization.Organization, *creditgrant.CreditGrant) {
	t.Helper()
	org := moneyOrg(name)
	db := datastore.New(org.Namespaced(ctx))
	m := meter.New(db)
	m.Name, m.EventName, m.AggregationType = "API usage", "api-usage", meter.AggSum
	if err := m.Create(); err != nil {
		t.Fatalf("meter: %v", err)
	}
	r := pricingrule.New(db)
	r.MeterId, r.PricingType, r.UnitPrice = m.Id(), pricingrule.PerUnit, 3
	if err := r.Create(); err != nil {
		t.Fatalf("pricing rule: %v", err)
	}
	e := meter.NewEvent(db)
	e.MeterId, e.UserId, e.Value, e.Timestamp = m.Id(), subject, 1000, time.Now()
	if err := e.Create(); err != nil {
		t.Fatalf("event: %v", err)
	}
	return org, grant(t, db, subject, 1000)
}

func grantHolds(t *testing.T, g *creditgrant.CreditGrant) int64 {
	t.Helper()
	if err := g.GetById(g.Id()); err != nil {
		t.Fatalf("reload grant: %v", err)
	}
	return g.RemainingCents
}

func readPreview(t *testing.T, org *organization.Organization, ctx context.Context, method, path, body string, h zip.Handler) map[string]any {
	t.Helper()
	app := zip.New(zip.Config{DisableStartupMessage: true})
	route, _, _ := strings.Cut(path, "?")
	app.Raw(method, route, func(c *zip.Ctx) error {
		c.Locals("organization", org)
		c.SetContext(ctx)
		return c.Next()
	}, h)
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%s %s: status=%d body=%s", method, path, resp.StatusCode, raw)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("bad json: %v (%s)", err, raw)
	}
	return out
}

// Viewing a preview leaves every credit grant as it was, however many times it is
// viewed, and still reports what the grants would cover.
func TestInvoicePreviewSpendsNoCredit(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	const subject = "preview-inv/alice"
	org, g := previewOrg(t, ctx, "preview-inv", subject)

	for i := 0; i < 3; i++ {
		out := readPreview(t, org, ctx, http.MethodPost, "/v1/billing/invoice-preview", `{"userId":"`+subject+`"}`, InvoicePreview)
		if out["subtotal"].(float64) != 3000 || out["creditApplied"].(float64) != 1000 || out["amountDue"].(float64) != 2000 {
			t.Fatalf("preview %d = %v, want subtotal 3000, creditApplied 1000, amountDue 2000", i, out)
		}
		if left := grantHolds(t, g); left != 1000 {
			t.Fatalf("after preview %d the grant holds %d, want 1000: the preview spent credit", i, left)
		}
	}
}

func TestUpcomingInvoiceSpendsNoCredit(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	const subject = "preview-up/alice"
	org, g := previewOrg(t, ctx, "preview-up", subject)

	for i := 0; i < 3; i++ {
		out := readPreview(t, org, ctx, http.MethodGet, "/v1/billing/invoices/upcoming?userId="+subject, "", UpcomingInvoice)
		if out["creditApplied"].(float64) != 1000 {
			t.Fatalf("upcoming %d = %v, want creditApplied 1000", i, out)
		}
		if left := grantHolds(t, g); left != 1000 {
			t.Fatalf("after upcoming %d the grant holds %d, want 1000", i, left)
		}
	}
}

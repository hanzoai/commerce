package payables

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

	"github.com/hanzoai/commerce/auth"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/fee"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/transfer"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/commerce/util/test/ae"
)

// seed installs the org + a platform-admin IAM claim (owner "admin" is what
// IsSuperAdmin checks, which is what RequirePlatformAdmin demands).
func seed(ns string) zip.Handler {
	return func(c *zip.Ctx) error {
		org := &organization.Organization{}
		org.Name = ns
		c.Locals("organization", org)
		c.SetContext(nscontext.WithNamespace(context.Background(), ns))
		c.Locals("iam_authenticated", true)
		claims := &auth.IAMClaims{Owner: "admin", IsAdmin: true}
		claims.Subject = "z@hanzo.ai"
		c.Locals("iam_claims", claims)
		return c.Next()
	}
}

func do(t *testing.T, app *zip.App, method, path string, body any) (int, []byte) {
	t.Helper()
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, r)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Fiber().Test(req)
	if err != nil {
		t.Fatalf("test request: %v", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b
}

func callList(t *testing.T, ns string) (int, ListResponse) {
	t.Helper()
	app := zip.New(zip.Config{DisableStartupMessage: true})
	app.Get("/payables", seed(ns), List)
	code, body := do(t, app, http.MethodGet, "/payables", nil)
	var out ListResponse
	_ = json.Unmarshal(body, &out)
	return code, out
}

func callPay(t *testing.T, ns, id string, req paymentRequest) (int, []byte) {
	t.Helper()
	app := zip.New(zip.Config{DisableStartupMessage: true})
	app.Post("/payables/:feeid/payments", seed(ns), RecordPayment)
	return do(t, app, http.MethodPost, "/payables/"+id+"/payments", req)
}

// seedFee accrues one payable in org ns, backdated by age.
func seedFee(t *testing.T, ns string, cents int64, age time.Duration) *fee.Fee {
	t.Helper()
	db := datastore.New(nscontext.WithNamespace(context.Background(), ns))
	f := fee.New(db)
	f.Name = "Referral revenue share"
	f.Type = fee.Affiliate
	f.PayeeId = "aff-1"
	f.Currency = "usd"
	f.Amount = currency.Cents(cents)
	f.Status = fee.Pending
	if err := f.Create(); err != nil {
		t.Fatalf("create fee: %v", err)
	}
	if age > 0 {
		f.CreatedAt = time.Now().UTC().Add(-age)
		if err := f.Update(); err != nil {
			t.Fatalf("backdate fee: %v", err)
		}
	}
	return f
}

// A pending payable past the clawback buffer matures to payable; a fresh one
// does not. This is the transition nothing in the repo ever performed.
func TestList_PayableMatures(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()
	const ns = "matures"

	old := seedFee(t, ns, 25000, 60*24*time.Hour) // 60 days: past the 30-day buffer
	fresh := seedFee(t, ns, 5000, 0)              // today: still clawback-able

	code, out := callList(t, ns)
	if code != 200 {
		t.Fatalf("list: got %d", code)
	}
	if out.Matured != 1 {
		t.Fatalf("matured: got %d, want 1", out.Matured)
	}

	got := map[string]fee.Status{}
	for _, p := range out.Payables {
		got[p.Id] = p.Status
	}
	if got[old.Id()] != fee.Payable {
		t.Errorf("aged fee: got %q, want payable", got[old.Id()])
	}
	if got[fresh.Id()] != fee.Pending {
		t.Errorf("fresh fee: got %q, want pending", got[fresh.Id()])
	}
	if len(out.Totals) != 1 || out.Totals[0].Amount != "300.00" || out.Totals[0].Asset != "usd" {
		t.Errorf("total owed: got %+v, want 300.00 usd", out.Totals)
	}
}

// Recording a payment settles the payable and writes the annotation. An ETH
// payment carries the exact amount sent as a decimal string — never in a cents
// field, which would truncate wei to zero.
func TestRecordPayment_Settles(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()
	const ns = "settles"

	f := seedFee(t, ns, 25000, 60*24*time.Hour)

	code, body := callPay(t, ns, f.Id(), paymentRequest{
		Method:    "eth",
		Reference: "0xabc123",
		Settles:   "100.00", // partial: $100.00 of $250.00
		Sent:      currency.Money{Amount: "0.0731", Asset: "eth"},
		Note:      "sent from treasury",
	})
	if code != 201 {
		t.Fatalf("record payment: got %d: %s", code, body)
	}

	var tr transfer.Transfer
	if err := json.Unmarshal(body, &tr); err != nil {
		t.Fatalf("decode transfer: %v", err)
	}
	if tr.Type != transfer.ETH || tr.Reference != "0xabc123" {
		t.Errorf("annotation: got %q/%q, want eth/0xabc123", tr.Type, tr.Reference)
	}
	if tr.Sent.Amount != "0.0731" || tr.Sent.Asset != "eth" {
		t.Errorf("sent: got %+v, want 0.0731 eth", tr.Sent)
	}
	if tr.Settles.Amount != "100.00" || tr.Settles.Asset != "usd" {
		t.Errorf("settles: got %+v, want 100.00 usd", tr.Settles)
	}
	if tr.Actor != "z@hanzo.ai" {
		t.Errorf("actor: got %q", tr.Actor)
	}

	// Partially paid: outstanding is the fold, and the payable still shows up.
	_, out := callList(t, ns)
	if len(out.Payables) != 1 {
		t.Fatalf("payables after partial payment: got %d, want 1", len(out.Payables))
	}
	if got := out.Payables[0].Outstanding; got.Amount != "150.00" || got.Asset != "usd" {
		t.Errorf("outstanding: got %+v, want 150.00 usd", got)
	}

	// Paying the rest clears it — a second payment on the same payable, in a
	// different method, with no special case.
	if code, body := callPay(t, ns, f.Id(), paymentRequest{
		Method: "wire", Reference: "WIRE-99", Settles: "150.00",
	}); code != 201 {
		t.Fatalf("second payment: got %d: %s", code, body)
	}
	if _, out := callList(t, ns); len(out.Payables) != 0 {
		t.Errorf("fully settled payable still owed: %+v", out.Payables)
	}
}

// The same real-world payment recorded twice settles once. (method, reference)
// names one payment.
func TestRecordPayment_DuplicateReferenceRefused(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()
	const ns = "dupe"

	f := seedFee(t, ns, 25000, 60*24*time.Hour)
	req := paymentRequest{Method: "eth", Reference: "0xdeadbeef", Settles: "100.00"}

	if code, body := callPay(t, ns, f.Id(), req); code != 201 {
		t.Fatalf("first payment: got %d: %s", code, body)
	}
	// Replay: accepted, but not settled again.
	if code, body := callPay(t, ns, f.Id(), req); code != 200 {
		t.Fatalf("replay: got %d, want 200 (idempotent): %s", code, body)
	}

	// Settled once, not twice.
	_, out := callList(t, ns)
	if len(out.Payables) != 1 || out.Payables[0].Paid.Amount != "100.00" {
		t.Errorf("double-settled: %+v", out.Payables)
	}

	db := datastore.New(nscontext.WithNamespace(context.Background(), ns))
	trs := make([]*transfer.Transfer, 0)
	if _, err := transfer.Query(db).Filter("Reference=", "0xdeadbeef").GetAll(&trs); err != nil {
		t.Fatalf("query transfers: %v", err)
	}
	if len(trs) != 1 {
		t.Errorf("payment records: got %d, want 1", len(trs))
	}
}

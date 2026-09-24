package billing

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/util/test/ae"
)

// TestListInvoices_PagesNewestFirst pins the listing's paging contract: rows
// come back newest first, in pages of at most InvoicePageMax, the cursor
// advances page to page, the final short page carries none, and a malformed
// cursor is a refusal, not a guess.
func TestListInvoices_PagesNewestFirst(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()
	org := moneyOrg("invoices-page")
	db := datastore.New(org.Namespaced(tc))

	base := time.Now().Add(-time.Hour).Round(time.Minute)
	for i := 0; i < 7; i++ {
		inv := billinginvoice.New(db)
		inv.UserId = "invoices-page/alice"
		inv.Status = billinginvoice.Open
		inv.AmountDue = int64(1000 + i)
		inv.CreatedAt = base.Add(time.Duration(i) * time.Minute)
		if err := inv.Create(); err != nil {
			t.Fatalf("seed invoice %d: %v", i, err)
		}
	}

	get := func(query string) (int, map[string]any) {
		t.Helper()
		w := driveSeeded(
			func(c *zip.Ctx) { c.Locals("organization", org) },
			"/v1/billing/invoices",
			httptest.NewRequest(http.MethodGet, "/v1/billing/invoices"+query, nil),
			ListBillingInvoices)
		b, _ := io.ReadAll(w.Body)
		var out map[string]any
		if err := json.Unmarshal(b, &out); err != nil {
			t.Fatalf("status %d body %s is not the listing shape: %v", w.StatusCode, b, err)
		}
		return w.StatusCode, out
	}

	amounts := func(out map[string]any) []int64 {
		var got []int64
		for _, row := range out["invoices"].([]any) {
			got = append(got, int64(row.(map[string]any)["amountDue"].(float64)))
		}
		return got
	}

	// The default page holds every row, newest first, and no next cursor.
	status, p1 := get("")
	if status != 200 {
		t.Fatalf("default page: status %d", status)
	}
	if got := amounts(p1); len(got) != 7 || got[0] != 1006 || got[6] != 1000 {
		t.Fatalf("default page amounts %v, want 1006..1000 newest first", got)
	}
	if _, more := p1["cursor"]; more {
		t.Fatalf("a short page carries a next cursor: %v", p1)
	}

	// 3-row pages: the cursor walks 1006/1005/1004, then 1003/1002/1001,
	// then the one short row, which ends the walk.
	status, p2 := get("?limit=3")
	if status != 200 {
		t.Fatalf("page one: status %d", status)
	}
	if got := amounts(p2); len(got) != 3 || got[0] != 1006 || got[2] != 1004 {
		t.Fatalf("page one amounts %v", got)
	}
	cursor, _ := p2["cursor"].(string)
	if cursor == "" {
		t.Fatalf("a full page carries no next cursor: %v", p2)
	}

	status, p3 := get("?limit=3&cursor=" + cursor)
	if status != 200 {
		t.Fatalf("page two: status %d", status)
	}
	if got := amounts(p3); len(got) != 3 || got[0] != 1003 {
		t.Fatalf("page two amounts %v", got)
	}
	cursor2, _ := p3["cursor"].(string)
	if cursor2 == "" {
		t.Fatalf("page two carries no next cursor: %v", p3)
	}

	status, p4 := get("?limit=3&cursor=" + cursor2)
	if status != 200 {
		t.Fatalf("page three: status %d", status)
	}
	if got := amounts(p4); len(got) != 1 || got[0] != 1000 {
		t.Fatalf("page three amounts %v", got)
	}
	if _, more := p4["cursor"]; more {
		t.Fatalf("the final short page carries a cursor: %v", p4)
	}

	// A limit past the max is the max, not a refusal; a bad cursor is a refusal.
	status, _ = get("?limit=999999")
	if status != 200 {
		t.Fatalf("limit past the max: status %d", status)
	}
	status, _ = get("?cursor=not-a-cursor")
	if status != http.StatusBadRequest {
		t.Fatalf("malformed cursor: status %d, want 400", status)
	}
}

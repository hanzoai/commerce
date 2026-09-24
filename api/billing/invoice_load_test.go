package billing

import (
	"net/http"
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/util/test/ae"
)

// TestPayInvoice_KeepsTheInvoiceListed: paying an invoice writes it back, and
// it stays in the invoice list. The list reads invoices under no ancestor, so a
// row read by id and written back is listed like any other.
func TestPayInvoice_KeepsTheInvoiceListed(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("inv-listed")
	withFakeSquare(t, squareMock("", "", "sqpay_listed"))
	db := datastore.New(org.Namespaced(ctx))
	sub := seedCardBackedSub(t, db, "inv-listed", "dev", "ccof_il", "cust_il")
	inv := seedOpenInvoice(t, db, sub, 2000)

	if resp := invokePay(org, ctx, inv.Id()); resp.StatusCode != http.StatusOK {
		t.Fatalf("pay status=%d, want 200", resp.StatusCode)
	}
	listed, err := ListInvoices(ctx, org, "", "", sub.Id(), 0, 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(listed) != 1 || listed[0].ID != inv.Id() || listed[0].Status != "paid" {
		t.Fatalf("invoice list after paying = %+v, want the one invoice, paid", listed)
	}
}

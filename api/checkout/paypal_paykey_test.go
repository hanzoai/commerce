package checkout

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/order"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/payment"
	"github.com/hanzoai/commerce/util/test/ae"
)

// A PayPal confirm or cancel names the pay key and no order id: the order is the
// one the payment holding that key was made for, and a key no payment holds is a 404.
func TestPayPalConfirmAndCancelFindTheOrderByPayKey(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	org := &organization.Organization{Name: "acme"}
	db := datastore.NewNamespaced(org.Namespaced(context.Background()))

	ord := order.New(db)
	ord.Type = "paypal"
	ord.MustPut()
	pay := payment.New(db)
	pay.OrderId = ord.Id()
	pay.Account.PayKey = "PAY-1"
	pay.MustPut()

	app := zip.New(zip.Config{DisableStartupMessage: true})
	asOrg := func(h zip.Handler) zip.Handler {
		return func(c *zip.Ctx) error {
			c.Locals("organization", org)
			return h(c)
		}
	}
	app.Raw(http.MethodPost, "/confirm/:payKey", asOrg(Confirm))
	app.Raw(http.MethodPost, "/cancel/:payKey", asOrg(Cancel))

	post := func(path string) (int, string) {
		t.Helper()
		resp, err := app.Test(httptest.NewRequest(http.MethodPost, path, nil))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		raw, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(raw)
	}

	if got, _ := post("/confirm/PAY-unknown"); got != http.StatusNotFound {
		t.Fatalf("confirm with an unknown pay key: status %d, want 404", got)
	}
	got, body := post("/confirm/PAY-1")
	if got != http.StatusOK {
		t.Fatalf("confirm with its pay key: status %d, want 200 (%s)", got, body)
	}
	var confirmed struct {
		Id            string         `json:"id"`
		PaymentStatus payment.Status `json:"paymentStatus"`
	}
	if err := json.Unmarshal([]byte(body), &confirmed); err != nil {
		t.Fatal(err)
	}
	if confirmed.Id != ord.Id() || confirmed.PaymentStatus != payment.Paid {
		t.Fatalf("confirm answered order %q %q, want %q %q", confirmed.Id, confirmed.PaymentStatus, ord.Id(), payment.Paid)
	}
	if got, body := post("/cancel/PAY-1"); got != http.StatusOK {
		t.Fatalf("cancel with its pay key: status %d, want 200 (%s)", got, body)
	}
}

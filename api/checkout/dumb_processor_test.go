package checkout

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Stripe is a processor of PaymentIntents (api/billing), never a host of checkout
// pages, coupons or catalogs. This fails if a hosted-checkout path to Stripe's
// Checkout Sessions, Coupons, Prices or Products comes back into this package.
func TestHostedCheckoutNeverCallsStripeBillingObjects(t *testing.T) {
	files, _ := filepath.Glob("*.go")
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, bad := range []string{"api.stripe.com", "checkout/sessions\",", "stripeprovider", "/v1/coupons"} {
			if strings.Contains(string(b), bad) {
				t.Errorf("%s reaches Stripe-hosted objects (%q); Stripe takes PaymentIntents only", f, bad)
			}
		}
	}
}

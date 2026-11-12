package checkout

import (
	"context"
	"testing"

	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
)

// A hosted checkout picks its processor by the one rule, over the merchant's
// own accounts: Square wherever Square takes the payment, Stripe where it does
// not or where the caller names it, and nothing on an account the merchant
// does not hold.
func TestCheckoutProcessors_Rule(t *testing.T) {
	org := &organization.Organization{Live: true}
	org.Name = "merchant"
	org.Square.Production.AccessToken = "sq-own"
	org.Square.Production.LocationId = "LOC-OWN"
	org.Stripe.Live.AccessToken = "sk_live_own"
	reg := checkoutProcessors(org)

	pick := func(cur currency.Type, country string, want processor.ProcessorType) processor.ProcessorType {
		t.Helper()
		p, err := reg.Fiat(context.Background(), processor.Need{Currency: cur, Country: country, Method: "card", Want: want})
		if err != nil {
			return ""
		}
		return p.Type()
	}
	for _, c := range []struct {
		cur     currency.Type
		country string
		want    processor.ProcessorType
		got     processor.ProcessorType
	}{
		{currency.USD, "US", "", processor.Square},
		{currency.USD, "", "", processor.Square},
		{currency.CHF, "US", "", processor.Stripe},
		{currency.USD, "DE", "", processor.Stripe},
		{currency.USD, "US", processor.Stripe, processor.Stripe},
		{currency.CHF, "US", processor.Square, ""}, // named Square cannot take CHF: no substitute
	} {
		if got := pick(c.cur, c.country, c.want); got != c.got {
			t.Errorf("%s from %q naming %q: got %q, want %q", c.cur, c.country, c.want, got, c.got)
		}
	}

	// A live merchant with no Stripe key of its own has no Stripe here, and the
	// deployment's never stands in for it.
	t.Setenv("STRIPE_SECRET_KEY", "sk_live_deployment")
	bare := &organization.Organization{Live: true}
	bare.Square = org.Square
	if p, err := checkoutProcessors(bare).Fiat(context.Background(), processor.Need{Currency: currency.CHF, Method: "card"}); err == nil {
		t.Fatalf("CHF for a merchant with no Stripe account chose %q", p.Type())
	}
}

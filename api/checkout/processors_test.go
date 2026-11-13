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
	t.Setenv("COMMERCE_CHARGE_PROCESSORS", "*")
	org := &organization.Organization{Live: true}
	org.Name = "merchant"
	org.Square.Production.AccessToken = "sq-own"
	org.Square.Production.LocationId = "LOC-OWN"
	// A Stripe key on the merchant's record opens no hosted checkout: Stripe takes
	// PaymentIntents, not pages.
	org.Stripe.Live.AccessToken = "sk_live_own"
	reg := checkoutProcessors(org)

	pick := func(want processor.ProcessorType) processor.ProcessorType {
		t.Helper()
		p, err := reg.Fiat(context.Background(), processor.Need{Currency: currency.USD, Method: "card", Want: want})
		if err != nil {
			return ""
		}
		return p.Type()
	}
	if got := pick(processor.Square); got != processor.Square {
		t.Errorf("Square: got %q", got)
	}
	if got := pick(processor.Stripe); got != "" {
		t.Errorf("a hosted checkout offered Stripe (%q)", got)
	}
	if p, err := reg.Fiat(context.Background(), processor.Need{Currency: currency.CHF, Method: "card"}); err == nil {
		t.Fatalf("CHF, which Square cannot take, chose %q", p.Type())
	}
}

// Hosted checkout does not gate Square on the buyer's country (Square's countries
// are where a SELLER can be), but a sanctioned buyer is refused.
func TestSanctionedOnly(t *testing.T) {
	t.Setenv("COMMERCE_CHARGE_PROCESSORS", "*")
	for in, want := range map[string]string{"DE": "", "BR": "", "": "", "US": "", "IR": "IR", "ru": "ru"} {
		if got := sanctionedOnly(in); got != want {
			t.Errorf("sanctionedOnly(%q) = %q; want %q", in, got, want)
		}
	}
	org := &organization.Organization{Live: true}
	org.Square.Production.AccessToken = "sq-own"
	org.Square.Production.LocationId = "LOC-OWN"
	reg := checkoutProcessors(org)
	if p, err := reg.Fiat(context.Background(), processor.Need{Currency: currency.USD, Country: sanctionedOnly("DE"), Method: "card"}); err != nil || p.Type() != processor.Square {
		t.Fatalf("a Square-only merchant, buyer in DE: %v %v; want Square", p, err)
	}
	if _, err := reg.Fiat(context.Background(), processor.Need{Currency: currency.USD, Country: sanctionedOnly("IR"), Method: "card"}); err == nil {
		t.Fatal("a buyer in IR was taken")
	}
}

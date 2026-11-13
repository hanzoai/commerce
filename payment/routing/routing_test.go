package routing

import (
	"fmt"
	"testing"

	"github.com/hanzoai/commerce/payment/processor"
)

func TestTheSameBuyerAlwaysLandsOnTheSameProcessor(t *testing.T) {
	p := Default()
	for i := 0; i < 500; i++ {
		in := Input{Org: "hanzo", Key: fmt.Sprintf("user-%d", i), Country: "US"}
		first := p.Decide(in).Processor
		for j := 0; j < 5; j++ {
			if got := p.Decide(in).Processor; got != first {
				t.Fatalf("%s moved from %s to %s", in.Key, first, got)
			}
		}
	}
}

func TestTheSplitIsAboutFiftyFiftyOverTenThousandKeys(t *testing.T) {
	p := Default()
	sq := 0
	for i := 0; i < 10000; i++ {
		if p.Decide(Input{Org: "hanzo", Key: fmt.Sprintf("u%d", i), Country: "GB"}).Processor == processor.Square {
			sq++
		}
	}
	if sq < 4800 || sq > 5200 {
		t.Fatalf("square took %d of 10000, want about 5000", sq)
	}
}

func TestRaisingTheWeightOnlyMovesBuyersTowardSquare(t *testing.T) {
	lo, hi := Default(), Default()
	lo.SquareWeight, hi.SquareWeight = 30, 70
	for i := 0; i < 2000; i++ {
		in := Input{Org: "hanzo", Key: fmt.Sprintf("u%d", i), Country: "US"}
		if lo.Decide(in).Processor == processor.Square && hi.Decide(in).Processor != processor.Square {
			t.Fatalf("%s left Square when the weight rose", in.Key)
		}
	}
}

func TestACountryOutsideSquaresEightGoesToStripe(t *testing.T) {
	p := Default()
	p.SquareWeight = 100
	for _, c := range []string{"DE", "BR", "NL", "MX", "IN", "SG", "PL", ""} {
		d := p.Decide(Input{Org: "hanzo", Key: "k", Country: c})
		if d.Processor != processor.Stripe || d.Reason != "country" {
			t.Fatalf("%q went to %s (%s)", c, d.Processor, d.Reason)
		}
	}
	for _, c := range SquareCountries {
		if p.Decide(Input{Org: "hanzo", Key: "k", Country: c}).Processor != processor.Square {
			t.Fatalf("%s did not go to Square at weight 100", c)
		}
	}
}

func TestASavedCardSticksToItsProcessor(t *testing.T) {
	p := Default()
	p.SquareWeight = 100
	if d := p.Decide(Input{Org: "hanzo", Key: "k", Country: "US", Saved: processor.Stripe}); d.Processor != processor.Stripe || d.Reason != "saved" {
		t.Fatalf("a Stripe card moved to %s (%s)", d.Processor, d.Reason)
	}
	p.SquareWeight, p.StripeOn = 0, false
	if d := p.Decide(Input{Org: "hanzo", Key: "k", Country: "DE", Saved: processor.Square}); d.Processor != processor.Square {
		t.Fatalf("a Square card moved to %s", d.Processor)
	}
}

func TestAProcessorSwitchedOffTakesNoNewCheckout(t *testing.T) {
	p := Default()
	p.SquareOn = false
	if d := p.Decide(Input{Org: "hanzo", Key: "k", Country: "US"}); d.Processor != processor.Stripe {
		t.Fatalf("square off, got %s", d.Processor)
	}
	p = Default()
	p.StripeOn = false
	if d := p.Decide(Input{Org: "hanzo", Key: "k", Country: "BR"}); d.Processor != processor.Square {
		t.Fatalf("stripe off, got %s", d.Processor)
	}
}

func TestMethods(t *testing.T) {
	p := Default()
	if m := p.Methods(processor.Square, "US"); m[len(m)-1] != "cash_app" {
		t.Fatalf("Cash App Pay is Square's in the US: %v", m)
	}
	if m := p.Methods(processor.Square, "GB"); len(m) != 3 {
		t.Fatalf("no Cash App Pay outside the US: %v", m)
	}
	if m := p.Methods(processor.Stripe, "BR"); m[0] != "payment_element" {
		t.Fatalf("Stripe mounts the Payment Element: %v", m)
	}
	p.LocalOn = false
	if m := p.Methods(processor.Stripe, "BR"); m[0] != "card" {
		t.Fatalf("local off mounts the card form: %v", m)
	}
}

func TestASavedStripeCardIsOfferedTheCardFormNotThePaymentElement(t *testing.T) {
	d := Default().Decide(Input{Org: "hanzo", Key: "k", Country: "BR", Saved: processor.Stripe})
	if d.Methods[0] != "card" {
		t.Fatalf("a Stripe card holder was sent to %v", d.Methods)
	}
}

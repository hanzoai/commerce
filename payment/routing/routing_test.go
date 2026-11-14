package routing

import (
	"fmt"
	"testing"

	"github.com/hanzoai/commerce/payment/processor"
)

func TestTheSameBuyerAlwaysLandsOnTheSameProcessor(t *testing.T) {
	p := Default()
	for i := 0; i < 500; i++ {
		in := Input{Org: "hanzo", Key: fmt.Sprintf("user-%d", i), Country: "US", SquareOK: true}
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
		if p.Decide(Input{Org: "hanzo", Key: fmt.Sprintf("u%d", i), Country: "GB", SquareOK: true}).Processor == processor.Square {
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
		in := Input{Org: "hanzo", Key: fmt.Sprintf("u%d", i), Country: "US", SquareOK: true}
		if lo.Decide(in).Processor == processor.Square && hi.Decide(in).Processor != processor.Square {
			t.Fatalf("%s left Square when the weight rose", in.Key)
		}
	}
}

func TestTheMerchantDecidesAndTheBuyersCountryDoesNot(t *testing.T) {
	p := Default()
	p.SquareWeight = 100
	// A merchant account Square can charge: any buyer's country is eligible, a German
	// buyer paying dollars to a US location included.
	for _, c := range []string{"US", "DE", "BR", "NL", "IN", ""} {
		if d := p.Decide(Input{Org: "hanzo", Key: "k", Country: c, SquareOK: true}); d.Processor != processor.Square {
			t.Fatalf("%q with an eligible merchant went to %s (%s)", c, d.Processor, d.Reason)
		}
	}
	// A merchant account Square cannot charge (the currency is not the location's):
	// Stripe's, from every country, decided before any payment exists.
	for _, c := range []string{"US", "DE", "FR", ""} {
		if d := p.Decide(Input{Org: "hanzo", Key: "k", Country: c, SquareOK: false}); d.Processor != processor.Stripe || d.Reason != "merchant" {
			t.Fatalf("%q with no eligible merchant went to %s (%s)", c, d.Processor, d.Reason)
		}
	}
}

func TestASavedCardSticksToItsProcessor(t *testing.T) {
	p := Default()
	p.SquareWeight = 100
	if d := p.Decide(Input{Org: "hanzo", Key: "k", Country: "US", Saved: processor.Stripe, SquareOK: true}); d.Processor != processor.Stripe || d.Reason != "saved" {
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
	if d := p.Decide(Input{Org: "hanzo", Key: "k", Country: "US", SquareOK: true}); d.Processor != processor.Stripe {
		t.Fatalf("square off, got %s", d.Processor)
	}
	p = Default()
	p.StripeOn = false
	if d := p.Decide(Input{Org: "hanzo", Key: "k", Country: "BR", SquareOK: true}); d.Processor != processor.Square {
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

func TestAnAssignmentIsAPreferenceNotARoute(t *testing.T) {
	p := Default()
	p.SquareWeight = 0 // the weight has since moved to Stripe; nobody placed moves
	d := p.Decide(Input{Org: "hanzo", Key: "k", Country: "DE", Assigned: processor.Square, SquareOK: true})
	if d.Processor != processor.Square || d.Reason != "assigned" {
		t.Fatalf("a Square-assigned customer with an eligible merchant was moved: %+v", d)
	}
	// Square cannot take this checkout (another currency, or unreachable): Stripe takes it.
	d = p.Decide(Input{Org: "hanzo", Key: "k", Country: "DE", Assigned: processor.Square, SquareOK: false})
	if d.Processor != processor.Stripe {
		t.Fatalf("a Square-assigned customer could not use Stripe while Square was ineligible: %+v", d)
	}
	// The assignment is the caller's to keep; the next eligible checkout is Square's again.
	d = p.Decide(Input{Org: "hanzo", Key: "k", Country: "DE", Assigned: processor.Square, SquareOK: true})
	if d.Processor != processor.Square {
		t.Fatalf("the assignment did not apply again at the next eligible checkout: %+v", d)
	}
	// A Stripe-assigned customer stays on Stripe whatever the weight now is.
	p.SquareWeight = 100
	if d := p.Decide(Input{Org: "hanzo", Key: "k", Country: "US", Assigned: processor.Stripe, SquareOK: true}); d.Processor != processor.Stripe {
		t.Fatalf("a Stripe-assigned customer was re-randomized: %+v", d)
	}
}

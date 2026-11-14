package routing

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hanzoai/commerce/payment/processor"
)

// sq is what the caller says of Square for one checkout.
func squareCan(ok bool) []Capability { return []Capability{{Processor: processor.Square, OK: ok}} }

// split sets the weights of Square and Stripe.
func split(p *Policy, square, stripe int) {
	for i := range p.Processors {
		switch p.Processors[i].Processor {
		case string(processor.Square):
			p.Processors[i].Weight = square
		case string(processor.Stripe):
			p.Processors[i].Weight = stripe
		}
	}
}

// switchOff takes a processor out of new checkouts.
func switchOff(p *Policy, t processor.ProcessorType) {
	for i := range p.Processors {
		if p.Processors[i].Processor == string(t) {
			p.Processors[i].On = false
		}
	}
}

func TestTheSameBuyerAlwaysLandsOnTheSameProcessor(t *testing.T) {
	p := Default()
	for i := 0; i < 500; i++ {
		in := Input{Org: "hanzo", Key: fmt.Sprintf("user-%d", i), Country: "US", Capable: squareCan(true)}
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
		if p.Decide(Input{Org: "hanzo", Key: fmt.Sprintf("u%d", i), Country: "GB", Capable: squareCan(true)}).Processor == processor.Square {
			sq++
		}
	}
	if sq < 4800 || sq > 5200 {
		t.Fatalf("square took %d of 10000, want about 5000", sq)
	}
}

func TestRaisingTheWeightOnlyMovesBuyersTowardSquare(t *testing.T) {
	lo, hi := Default(), Default()
	split(&lo, 30, 70)
	split(&hi, 70, 30)
	for i := 0; i < 2000; i++ {
		in := Input{Org: "hanzo", Key: fmt.Sprintf("u%d", i), Country: "US", Capable: squareCan(true)}
		if lo.Decide(in).Processor == processor.Square && hi.Decide(in).Processor != processor.Square {
			t.Fatalf("%s left Square when the weight rose", in.Key)
		}
	}
}

func TestTheMerchantDecidesAndTheBuyersCountryDoesNot(t *testing.T) {
	p := Default()
	split(&p, 100, 0)
	// A merchant account Square can charge: any buyer's country is eligible, a German
	// buyer paying dollars to a US location included.
	for _, c := range []string{"US", "DE", "BR", "NL", "IN", ""} {
		if d := p.Decide(Input{Org: "hanzo", Key: "k", Country: c, Capable: squareCan(true)}); d.Processor != processor.Square {
			t.Fatalf("%q with an eligible merchant went to %s (%s)", c, d.Processor, d.Reason)
		}
	}
	// A merchant account Square cannot charge (the currency is not the location's):
	// Stripe's, from every country, decided before any payment exists.
	for _, c := range []string{"US", "DE", "FR", ""} {
		if d := p.Decide(Input{Org: "hanzo", Key: "k", Country: c, Capable: squareCan(false)}); d.Processor != processor.Stripe || d.Reason != "merchant" {
			t.Fatalf("%q with no eligible merchant went to %s (%s)", c, d.Processor, d.Reason)
		}
	}
}

func TestASavedCardSticksToItsProcessor(t *testing.T) {
	p := Default()
	split(&p, 100, 0)
	if d := p.Decide(Input{Org: "hanzo", Key: "k", Country: "US", Saved: processor.Stripe, Capable: squareCan(true)}); d.Processor != processor.Stripe || d.Reason != "saved" {
		t.Fatalf("a Stripe card moved to %s (%s)", d.Processor, d.Reason)
	}
	split(&p, 0, 0)
	switchOff(&p, processor.Stripe)
	if d := p.Decide(Input{Org: "hanzo", Key: "k", Country: "DE", Saved: processor.Square}); d.Processor != processor.Square {
		t.Fatalf("a Square card moved to %s", d.Processor)
	}
}

func TestAProcessorSwitchedOffTakesNoNewCheckout(t *testing.T) {
	p := Default()
	switchOff(&p, processor.Square)
	if d := p.Decide(Input{Org: "hanzo", Key: "k", Country: "US", Capable: squareCan(true)}); d.Processor != processor.Stripe {
		t.Fatalf("square off, got %s", d.Processor)
	}
	p = Default()
	switchOff(&p, processor.Stripe)
	if d := p.Decide(Input{Org: "hanzo", Key: "k", Country: "BR", Capable: squareCan(true)}); d.Processor != processor.Square {
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
	split(&p, 0, 100) // the weight has since moved to Stripe; nobody placed moves
	d := p.Decide(Input{Org: "hanzo", Key: "k", Country: "DE", Assigned: processor.Square, Capable: squareCan(true)})
	if d.Processor != processor.Square || d.Reason != "assigned" {
		t.Fatalf("a Square-assigned customer with an eligible merchant was moved: %+v", d)
	}
	// Square cannot take this checkout (another currency, or unreachable): Stripe takes it.
	d = p.Decide(Input{Org: "hanzo", Key: "k", Country: "DE", Assigned: processor.Square, Capable: squareCan(false)})
	if d.Processor != processor.Stripe {
		t.Fatalf("a Square-assigned customer could not use Stripe while Square was ineligible: %+v", d)
	}
	// The assignment is the caller's to keep; the next eligible checkout is Square's again.
	d = p.Decide(Input{Org: "hanzo", Key: "k", Country: "DE", Assigned: processor.Square, Capable: squareCan(true)})
	if d.Processor != processor.Square {
		t.Fatalf("the assignment did not apply again at the next eligible checkout: %+v", d)
	}
	// A Stripe-assigned customer stays on Stripe whatever the weight now is.
	split(&p, 100, 0)
	if d := p.Decide(Input{Org: "hanzo", Key: "k", Country: "US", Assigned: processor.Stripe, Capable: squareCan(true)}); d.Processor != processor.Stripe {
		t.Fatalf("a Stripe-assigned customer was re-randomized: %+v", d)
	}
}

// three is the default policy with three card processors at 50/30/20.
func three() Policy {
	p := Default()
	p.Processors = []ProcessorRule{
		{string(processor.Square), true, 50}, {string(processor.Stripe), true, 30}, {string(processor.Adyen), true, 20},
		{string(processor.PayPal), true, 0}, {CryptoRail, true, 0},
	}
	return p
}

func TestThreeProcessorsAtFiftyThirtyTwentyShareBuyersAndKeepThem(t *testing.T) {
	p := three()
	if why := p.Valid(); why != "" {
		t.Fatal(why)
	}
	got := map[processor.ProcessorType]int{}
	for i := 0; i < 10000; i++ {
		in := Input{Org: "hanzo", Key: fmt.Sprintf("u%d", i), Country: "GB"}
		first := p.Decide(in)
		got[first.Processor]++
		if first.Reason != "split" {
			t.Fatalf("%s placed by %s, want the split", in.Key, first.Reason)
		}
		if i == 0 && !strings.Contains(first.Chain[5].Why, "square 50, stripe 30, adyen 20") {
			t.Fatalf("the reason chain does not show the shares: %+v", first.Chain[5])
		}
		for j := 0; j < 3; j++ {
			if again := p.Decide(in).Processor; again != first.Processor {
				t.Fatalf("%s moved from %s to %s", in.Key, first.Processor, again)
			}
		}
	}
	for proc, want := range map[processor.ProcessorType]int{processor.Square: 5000, processor.Stripe: 3000, processor.Adyen: 2000} {
		if got[proc] < want-250 || got[proc] > want+250 {
			t.Errorf("%s took %d of 10000, want about %d", proc, got[proc], want)
		}
	}
	// Placed once, a buyer stays where they were when the weights move.
	moved := three()
	split(&moved, 0, 100)
	in := Input{Org: "hanzo", Key: "u7", Country: "GB", Assigned: processor.Adyen}
	if d := moved.Decide(in); d.Processor != processor.Adyen || d.Reason != "assigned" {
		t.Errorf("a placed buyer was moved by the weights: %+v", d)
	}
}

func TestAnIneligibleProcessorIsSkippedAndTheRestShareItsBuyers(t *testing.T) {
	p := three()
	got := map[processor.ProcessorType]int{}
	for i := 0; i < 10000; i++ {
		d := p.Decide(Input{Org: "hanzo", Key: fmt.Sprintf("u%d", i), Country: "GB",
			Capable: []Capability{{Processor: processor.Adyen, OK: false, Why: "no Adyen account takes cards in EUR"}}})
		got[d.Processor]++
		if i == 0 && (!d.Chain[1].Applied || !strings.Contains(d.Chain[1].Why, "adyen excluded: no Adyen account takes cards in EUR")) {
			t.Fatalf("the reason chain does not say why Adyen was skipped: %+v", d.Chain[1])
		}
	}
	if got[processor.Adyen] != 0 {
		t.Fatalf("an ineligible processor took %d buyers", got[processor.Adyen])
	}
	// 50:30 renormalised: 62.5% and 37.5%.
	if got[processor.Square] < 6000 || got[processor.Square] > 6500 || got[processor.Stripe] < 3500 || got[processor.Stripe] > 4000 {
		t.Fatalf("the rest shared %v, want about 6250 and 3750", got)
	}
	// A switched-off processor is skipped the same way, and an account that does not
	// take the checkout's currency excludes its processor.
	off := three()
	switchOff(&off, processor.Square)
	if d := off.Decide(Input{Org: "hanzo", Key: "k", Currency: "usd"}); d.Processor == processor.Square {
		t.Fatalf("a switched-off processor took a checkout: %+v", d)
	}
	acct := three()
	acct.Accounts = []Account{{Processor: "square", ID: "L1", Country: "GB", Currency: "gbp", Cards: true, On: true}}
	for i := 0; i < 500; i++ {
		if d := acct.Decide(Input{Org: "hanzo", Key: fmt.Sprintf("a%d", i), Currency: "usd"}); d.Processor == processor.Square {
			t.Fatalf("a GBP-only Square account took a dollar checkout: %+v", d)
		}
	}
	if d := acct.Decide(Input{Org: "hanzo", Key: "a1", Currency: "gbp"}); d.Chain[1].Applied {
		t.Fatalf("Square's GBP account should take a GBP checkout: %+v", d.Chain[1])
	}
}

func TestAPolicyNeedsKnownDistinctProcessorsAndRealAccounts(t *testing.T) {
	for name, mut := range map[string]func(*Policy){
		"unknown processor":    func(p *Policy) { p.Processors = append(p.Processors, ProcessorRule{"venmo", true, 1}) },
		"duplicate processor":  func(p *Policy) { p.Processors = append(p.Processors, ProcessorRule{"stripe", true, 1}) },
		"nothing takes cards":  func(p *Policy) { switchOff(p, processor.Square); switchOff(p, processor.Stripe) },
		"account of no rule":   func(p *Policy) { p.Accounts = []Account{{Processor: "adyen", ID: "a", On: true, Cards: true}} },
		"account without id":   func(p *Policy) { p.Accounts = []Account{{Processor: "square", On: true}} },
		"account bad country":  func(p *Policy) { p.Accounts = []Account{{Processor: "square", ID: "L", Country: "ZZ"}} },
		"account bad currency": func(p *Policy) { p.Accounts = []Account{{Processor: "square", ID: "L", Currency: "dollars"}} },
	} {
		p := Default().Normal()
		mut(&p)
		if why := p.Normal().Valid(); why == "" {
			t.Errorf("%s was accepted", name)
		}
	}
}

// Copyright © 2026 Hanzo AI. MIT License.

package risk

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/hanzoai/commerce/models/screen"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/test/ae"
)

// facts_test.go regresses the durable-write hole: POST /screen stored ARBITRARY
// caller-supplied signals — any key, any length, any number of them — in the
// org's own permanent record. The allowlist existed but only on the processor
// path, so the typed API walked straight past it.

// TestFacts_TheStatedPathRefusesWhatThePlaneDoesNotRead — a caller that names a
// fact deliberately gets told it is not carried, rather than having it dropped
// and believing the screen saw it.
func TestFacts_TheStatedPathRefusesWhatThePlaneDoesNotRead(t *testing.T) {
	for _, key := range []string{"pan", "card_number", "ssn", "notes", "x"} {
		if _, err := Facts(map[string]string{key: "value"}); !errors.Is(err, ErrFact) {
			t.Fatalf("signal %q was accepted (err=%v)", key, err)
		}
	}
	got, err := Facts(map[string]string{"IP": " 203.0.113.7 ", "bin": "424242"})
	if err != nil {
		t.Fatalf("allowlisted facts were refused: %v", err)
	}
	if got["ip"] != "203.0.113.7" || got["bin"] != "424242" {
		t.Fatalf("normalization is wrong: %v", got)
	}
}

// TestFacts_TheStatedPathIsBounded — every accepted key lands in a durable row,
// so an open map is an unbounded write the caller controls.
func TestFacts_TheStatedPathIsBounded(t *testing.T) {
	if _, err := Facts(map[string]string{"ua": strings.Repeat("x", maxFact+1)}); !errors.Is(err, ErrFact) {
		t.Fatalf("an oversize signal value was accepted")
	}
	if _, err := Facts(map[string]string{"ua": strings.Repeat("x", maxFact)}); err != nil {
		t.Fatalf("a value at the bound was refused: %v", err)
	}

	many := map[string]string{}
	for i := 0; i <= maxStated; i++ {
		many[string(rune('a'+i%26))+string(rune('a'+i/26))] = "v"
	}
	if _, err := Facts(many); !errors.Is(err, ErrFact) {
		t.Fatalf("%d stated signals were walked without complaint", len(many))
	}
}

// TestFacts_TheMetadataPathDropsInsteadOfRefusing — a merchant's payment
// metadata is incidental, so an unread key must not fail a payment. Same
// allowlist, different disposition, one gate.
func TestFacts_TheMetadataPathDropsInsteadOfRefusing(t *testing.T) {
	got := Signals(map[string]any{
		"ip":          "203.0.113.7",
		"card_number": "4242424242424242",
		"ua":          strings.Repeat("x", maxFact+1),
		"note":        "a long free-text field the merchant uses for itself",
		"count":       42,
	})
	if got["ip"] != "203.0.113.7" {
		t.Fatalf("an allowlisted fact was dropped: %v", got)
	}
	if _, ok := got["card_number"]; ok {
		t.Fatalf("a card number reached the scoring plane: %v", got)
	}
	if _, ok := got["ua"]; ok {
		t.Fatalf("an oversize value was carried: %v", got)
	}
	if len(got) != 1 {
		t.Fatalf("metadata projection carried %d facts, want 1: %v", len(got), got)
	}
}

// TestFacts_ScreenRefusesAndStoresNothing — the gate is inside Screen, so EVERY
// entry point is covered by construction rather than by each caller
// remembering. A refused move writes no row.
func TestFacts_ScreenRefusesAndStoresNothing(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	s := tenant("factsgate", ctx, &oracle{answer: &Decision{Action: Allow}})
	_, err := s.Screen(context.Background(), Move{
		Stage: Payment, Subject: customer("c1"), Amount: 100, Currency: currency.USD,
		Signals: map[string]string{"pan": "4242424242424242"},
	})
	if !errors.Is(err, ErrFact) {
		t.Fatalf("an arbitrary signal was screened: %v", err)
	}
	if n := len(screen.For(s.DB, "", "", 0)); n != 0 {
		t.Fatalf("%d rows written for a refused move", n)
	}
	if p := s.Plane.(*oracle); p.count() != 0 {
		t.Fatalf("the scoring plane was asked %d times about a refused move", p.count())
	}
}

// TestFacts_TheMerchantStandingPassesTheSameGate — the counted standing travels
// the same wire into the same durable row, so it goes through the gate rather
// than around it. Two vocabularies, ONE allowlist.
func TestFacts_TheMerchantStandingPassesTheSameGate(t *testing.T) {
	st := &Standing{Subject: merchantOf("m1"), Window: 200, Reserved: map[string]int64{"usd": 5}}
	got, err := Facts(st.signals())
	if err != nil {
		t.Fatalf("the standing's own signals were refused by the gate: %v", err)
	}
	if got["window"] != "200" || got["reserved"] != "5" {
		t.Fatalf("the standing lost facts at the gate: %v", got)
	}
}

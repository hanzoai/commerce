// Copyright © 2026 Hanzo AI. MIT License.

package risk

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/hanzoai/commerce/models/control"
	"github.com/hanzoai/commerce/models/screen"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/test/ae"
)

// text_test.go is the gate on the defect class where a bound on the NUMBER of
// rows is mistaken for a bound on how much a read costs.
//
// Every read on this plane is capped at a row count. Every string on those rows
// used to be the caller's, at any length. 200 × ∞ is ∞, and the process that
// dies of it is shared with every other tenant.

// TestText_ACallerStringOverTheBoundIsRefusedNotTruncated — refused, and LOUDLY.
// A silently shortened reason is evidence altered with nobody told; a bound
// that binds quietly is a bound no operator can see binding.
func TestText_ACallerStringOverTheBoundIsRefusedNotTruncated(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	s := tenant("textbound", ctx, &oracle{answer: &Decision{Action: Allow}})
	m := merchant("m1")
	over := strings.Repeat("x", Text+1)

	for name, move := range map[string]Move{
		"reference": {Stage: Payout, Subject: m, Amount: 100, Currency: currency.USD, Reference: over},
		"processor": {Stage: Payout, Subject: m, Amount: 100, Currency: currency.USD, Processor: over},
		"idem":      {Stage: Payout, Subject: m, Amount: 100, Currency: currency.USD, Idem: over},
		"subject":   {Stage: Payout, Subject: Subject{Kind: KindMerchant, ID: over}, Amount: 100},
		"stage":     {Stage: Stage(over), Subject: m, Amount: 100},
	} {
		if _, err := s.Screen(context.Background(), move); err == nil {
			t.Fatalf("%s: an over-long caller string was accepted", name)
		}
	}
	if _, err := Place(s, Placement{Subject: m, Effect: control.Hold, Reason: over}); err == nil {
		t.Fatal("an over-long reason was accepted onto a declaration")
	}

	// Exactly at the bound is fine: the ceiling is a ceiling, not a fence.
	at := strings.Repeat("x", Text)
	if _, err := s.Screen(context.Background(), Move{
		Stage: Payout, Subject: m, Amount: 100, Currency: currency.USD, Reference: at, Idem: "ok",
	}); err != nil {
		t.Fatalf("a string exactly at the bound was refused: %v", err)
	}
}

// TestScreen_ARowCannotOutweighItsPublishedCeiling MEASURES the worst case.
//
// It builds the heaviest screen a caller can actually cause — every string at
// the bound, every allowlisted signal at the bound, the rule names at the
// bound, and the control ids of a tenant at control.Max declarations — marshals
// it the way the store does, and fails if [screen.Bytes] under-states what came
// out. A published ceiling nobody measured is a guess.
func TestScreen_ARowCannotOutweighItsPublishedCeiling(t *testing.T) {
	at := strings.Repeat("x", Text)

	// Every allowlisted signal, each at the bound.
	signals := map[string]string{}
	for k := range factKeys {
		signals[k] = at
	}
	// The most rule names a decision can record, each at the bound.
	rules := make([]string, Hits)
	for i := range rules {
		rules[i] = at
	}
	// A tenant with the most declarations it may hold, every one bearing.
	ids := make([]string, control.Max)
	for i := range ids {
		ids[i] = strings.Repeat("c", 24)
	}

	rec := &screen.Screen{}
	rec.Stage, rec.SubjectKind, rec.Subject = at, at, at
	rec.Currency = currency.Type(at)
	rec.Amount, rec.Held, rec.Allowed = 1<<62, 1<<62, 1<<62
	rec.Action, rec.Agency, rec.Decision, rec.Refusal, rec.Outcome = at, at, at, at, at
	rec.Reference, rec.Processor, rec.Reason, rec.Idem = at, at, at, at
	rec.Fingerprint, rec.Reserve = at, at
	rec.Detail = map[string]any{"signals": signals, "hits": rules, "controls": ids}

	body, err := json.Marshal(rec.Detail)
	if err != nil {
		t.Fatalf("marshal detail: %v", err)
	}
	weight := len(body)
	for _, f := range []string{
		rec.Stage, rec.SubjectKind, rec.Subject, string(rec.Currency),
		rec.Action, rec.Agency, rec.Decision, rec.Refusal, rec.Outcome,
		rec.Reference, rec.Processor, rec.Reason, rec.Idem, rec.Fingerprint, rec.Reserve,
	} {
		weight += len(f)
	}

	if weight > screen.Bytes {
		t.Fatalf("the worst-case row weighs %d bytes against a published ceiling of %d — "+
			"the ceiling under-states what one tenant can make a read cost (%d rows = %d bytes)",
			weight, screen.Bytes, screen.Max, weight*screen.Max)
	}
	t.Logf("worst-case row: %d bytes (ceiling %d); one full page: %d bytes",
		weight, screen.Bytes, weight*screen.Max)
}

// TestHits_AreBoundedInCountAndInSize — the scoring plane is ours, but it is
// reached at an address an operator configures, and a bound that holds only
// while the far end behaves is not a bound.
func TestHits_AreBoundedInCountAndInSize(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	huge := make([]string, Hits*10)
	for i := range huge {
		huge[i] = strings.Repeat("h", Text*4)
	}
	s := tenant("hitsbound", ctx, &oracle{answer: &Decision{Action: Allow, Hits: huge}})
	rec, err := s.Screen(context.Background(), Move{
		Stage: Payment, Subject: customer("c1"), Amount: 100, Currency: currency.USD, Idem: "h1",
	})
	if err != nil {
		t.Fatalf("screen: %v", err)
	}
	got, _ := rec.Detail["hits"].([]string)
	if len(got) != 0 {
		t.Fatalf("%d over-long rule names were stored", len(got))
	}

	// Bounded in COUNT as well as in size.
	many := make([]string, Hits*10)
	for i := range many {
		many[i] = "rule"
	}
	s2 := tenant("hitscount", ctx, &oracle{answer: &Decision{Action: Allow, Hits: many}})
	rec2, err := s2.Screen(context.Background(), Move{
		Stage: Payment, Subject: customer("c1"), Amount: 100, Currency: currency.USD, Idem: "h2",
	})
	if err != nil {
		t.Fatalf("screen: %v", err)
	}
	if got, _ := rec2.Detail["hits"].([]string); len(got) != Hits {
		t.Fatalf("%d rule names stored, want the %d bound", len(got), Hits)
	}
}

// TestScreen_ABoundThatBindsSaysSo — the third banned failure. A signal the
// allowlist drops and a rule list the ceiling truncates are inputs the score was
// NOT computed from; if the judgement records nothing, the plane has quietly
// scored on less than it was given and the only symptom is a flat score nobody
// can explain. So the count lands on the row an operator already reads.
func TestScreen_ABoundThatBindsSaysSo(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	over := strings.Repeat("h", Text*4)
	many := make([]string, Hits+7)
	for i := range many {
		many[i] = "rule"
	}
	many[0] = over // one over-long AND seven over the count

	s := tenant("loudbound", ctx, &oracle{answer: &Decision{Action: Allow, Hits: many}})
	rec, err := s.Screen(context.Background(), Move{
		Stage: Payment, Subject: customer("c1"), Amount: 100, Currency: currency.USD, Idem: "d1",
		Signals: map[string]string{
			"ip":       "1.2.3.4", // kept
			"email":    over,      // over the value bound
			"pan":      "4111",    // not on the allowlist
			"whatever": "x",       // not on the allowlist
		},
	})
	if err != nil {
		t.Fatalf("screen: %v", err)
	}

	lost, ok := rec.Detail["dropped"].(map[string]any)
	if !ok {
		t.Fatalf("three signals and eight rule names were dropped and the judgement "+
			"records no such thing: %v", rec.Detail)
	}
	if lost["signals"] != 3 {
		t.Fatalf("dropped signals recorded as %v, want 3", lost["signals"])
	}
	if lost["hits"] != 8 {
		t.Fatalf("dropped rule names recorded as %v, want 8", lost["hits"])
	}
	// And what survived is still exactly what the bounds allow.
	if got, _ := rec.Detail["signals"].(map[string]string); len(got) != 1 || got["ip"] != "1.2.3.4" {
		t.Fatalf("kept signals=%v, want just the one that fits", got)
	}
	// 39 arrived: 7 over the count, and of the 32 that fit the count one is over
	// the size — so 31 survive and 8 are accounted for.
	if got, _ := rec.Detail["hits"].([]string); len(got) != Hits-1 {
		t.Fatalf("%d rule names kept, want %d", len(got), Hits-1)
	}

	// A judgement whose inputs all fit says nothing, so the key means what it
	// says wherever it appears.
	tidy := tenant("quietbound", ctx, &oracle{answer: &Decision{Action: Allow, Hits: []string{"rule"}}})
	clean, err := tidy.Screen(context.Background(), Move{
		Stage: Payment, Subject: customer("c1"), Amount: 100, Currency: currency.USD, Idem: "d2",
		Signals: map[string]string{"ip": "1.2.3.4"},
	})
	if err != nil {
		t.Fatalf("screen: %v", err)
	}
	if _, seen := clean.Detail["dropped"]; seen {
		t.Fatalf("a judgement that dropped nothing reports %v", clean.Detail["dropped"])
	}
}

// Copyright © 2026 Hanzo AI. MIT License.

package risk

import (
	"context"
	"strings"
	"testing"

	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/test/ae"
)

// facts_test.go is the gate on durably storing whatever a caller sends.
//
// POST /v1/billing/risk/screen took a free-form map and wrote it into a record
// this plane keeps for years and forwards to a scoring plane. That is a store a
// caller can fill and a place a card number can land, so what may travel is a
// closed list with a size on it.

func TestFacts_KeepsOnlyAllowlistedKeys(t *testing.T) {
	got := Facts(map[string]string{
		"ip":       "203.0.113.7",
		"UA":       "curl/8",
		" email ":  "a@b.c",
		"card":     "4111111111111111",
		"cvv":      "123",
		"password": "hunter2",
		"note":     "anything at all",
	})
	want := map[string]string{"ip": "203.0.113.7", "ua": "curl/8", "email": "a@b.c"}
	if len(got) != len(want) {
		t.Fatalf("facts=%v want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("facts[%q]=%q want %q", k, got[k], v)
		}
	}
}

func TestFacts_DropsAValueTooLargeToBeTheFactItClaims(t *testing.T) {
	long := strings.Repeat("x", factMax+1)
	got := Facts(map[string]string{"ip": long, "ua": strings.Repeat("y", factMax), "email": ""})
	if _, ok := got["ip"]; ok {
		t.Fatalf("a %d-byte value travelled under the name of an ip", len(long))
	}
	if len(got["ua"]) != factMax {
		t.Fatalf("a value exactly at the bound was dropped")
	}
	if _, ok := got["email"]; ok {
		t.Fatal("an empty value travelled")
	}
}

// TestFacts_IsBoundedHoweverHardACallerPushes — the whole projection has a
// ceiling, so what one screen can write into the store does not depend on the
// caller's imagination.
func TestFacts_IsBoundedHoweverHardACallerPushes(t *testing.T) {
	in := map[string]string{}
	for i := 0; i < 5000; i++ {
		in[strings.Repeat("k", i%40+1)] = strings.Repeat("v", 300)
	}
	for k := range factKeys {
		in[k] = strings.Repeat("v", factMax)
	}
	got := Facts(in)
	if len(got) > len(factKeys) {
		t.Fatalf("%d facts travelled, want at most %d", len(got), len(factKeys))
	}
	size := 0
	for k, v := range got {
		size += len(k) + len(v)
	}
	if max := len(factKeys) * (factMax + 32); size > max {
		t.Fatalf("the projection is %d bytes, want at most %d", size, max)
	}
}

// TestSignals_IsTheSameAllowlistThroughTheProcessorDoor — two doors, one rule.
func TestSignals_IsTheSameAllowlistThroughTheProcessorDoor(t *testing.T) {
	got := Signals(map[string]any{
		"ip": "203.0.113.7", "card": "4111111111111111", "amount": 42, "ua": strings.Repeat("z", factMax+1),
	})
	if len(got) != 1 || got["ip"] != "203.0.113.7" {
		t.Fatalf("signals=%v — the processor door must apply the same list and the same bound", got)
	}
}

// TestScreen_StoresOnlyWhatMayTravel — the allowlist is applied INSIDE the
// screener, so a caller reaching it by any door gets the same treatment and no
// third caller can forget.
func TestScreen_StoresOnlyWhatMayTravel(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	p := &oracle{answer: &Decision{Action: Allow}}
	s := tenant("factstore", ctx, p)
	rec, err := s.Screen(context.Background(), Move{
		Stage: Payment, Subject: customer("c1"), Amount: 100, Currency: currency.USD,
		Signals: map[string]string{"ip": "203.0.113.7", "card": "4111111111111111", "junk": strings.Repeat("x", 5000)},
	})
	if err != nil {
		t.Fatalf("screen: %v", err)
	}
	stored, ok := rec.Detail["signals"].(map[string]string)
	if !ok {
		t.Fatalf("detail signals are %T", rec.Detail["signals"])
	}
	if len(stored) != 1 || stored["ip"] != "203.0.113.7" {
		t.Fatalf("stored=%v — only allowlisted facts are written to the record", stored)
	}
	if len(p.asks) != 1 {
		t.Fatalf("%d asks", len(p.asks))
	}
	if _, leaked := p.asks[0].Signals["card"]; leaked {
		t.Fatal("a card number reached the scoring plane")
	}
}

// TestScreen_ACallerCannotStateItsOwnStanding — the counted facts a merchant is
// judged on are ours. A caller that could send its own dispute rate would be
// scored on a number it chose, so the two travel in different fields and only
// one of them is reachable from a request.
func TestScreen_ACallerCannotStateItsOwnStanding(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	p := &oracle{answer: &Decision{Action: Allow}}
	s := tenant("standing", ctx, p)
	if _, err := s.Screen(context.Background(), Move{
		Stage:    Merchant,
		Subject:  merchant("m1"),
		Signals:  map[string]string{"disputerate": "0", "refused": "0", "ip": "203.0.113.7"},
		Standing: map[string]string{"disputerate": "4200", "refused": "17"},
	}); err != nil {
		t.Fatalf("screen: %v", err)
	}
	sent := p.asks[0].Signals
	if sent["disputerate"] != "4200" || sent["refused"] != "17" {
		t.Fatalf("the caller's claim beat the counted fact: %v", sent)
	}
	if sent["ip"] != "203.0.113.7" {
		t.Fatalf("a legitimate caller fact was lost: %v", sent)
	}
}

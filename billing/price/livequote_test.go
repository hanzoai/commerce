package price

import (
	"context"
	"os"
	"testing"
	"time"
)

// A LIVE read against the real venues, and the sibling of depositledger's
// livechain probes.
//
// Everything else here runs against stubs, which is right for the oracle's
// JUDGEMENT and useless for the question that decides whether a native coin may
// be credited at all: do these two venues actually answer, for these symbols,
// in a shape this code parses? A stub cannot fail the way a venue renames a
// field or retires a pair.
//
// Skipped by default — it reaches the public internet and its result depends on
// two third parties being up. Run it deliberately:
//
//	PRICE_LIVE=1 go test ./billing/price/ -run TestLiveQuote -v
func TestLiveQuote(t *testing.T) {
	if os.Getenv("PRICE_LIVE") == "" {
		t.Skip("set PRICE_LIVE=1 to read the real venues")
	}
	o, err := Default()
	if err != nil {
		t.Fatalf("Default: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// The three the rail is being asked to credit natively.
	for _, sym := range []string{"BTC", "XRP", "TON"} {
		q, err := o.Quote(ctx, sym)
		if err != nil {
			t.Errorf("%s: %v", sym, err)
			continue
		}
		t.Logf("%-4s $%s  (%d micro-cents)  spread=%.4f%%  sources=%v", sym, q, q.MicroCents, q.Spread*100, q.Sources)
		if len(q.Sources) < MinSources {
			t.Errorf("%s quoted by %v, want at least %d venues", sym, q.Sources, MinSources)
		}
		// A sanity floor, not a price assertion: BTC under $1,000 means a parse
		// landed on the wrong field, which is exactly the failure a stub cannot
		// produce.
		if sym == "BTC" && q.MicroCents < 100_000*Scale {
			t.Errorf("BTC quoted at %d micro-cents — that is not a BTC price, the parse is wrong", q.MicroCents)
		}
		if q.MicroCents <= 0 {
			t.Errorf("%s quoted %d micro-cents", sym, q.MicroCents)
		}
	}
}

// Each source must answer on its OWN, or a venue that has silently stopped
// serving a pair hides behind the other one until the day it is needed.
func TestLiveEachSourceAnswers(t *testing.T) {
	if os.Getenv("PRICE_LIVE") == "" {
		t.Skip("set PRICE_LIVE=1 to read the real venues")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	for _, s := range []Source{Coinbase(nil), Kraken(nil)} {
		for _, sym := range []string{"BTC", "XRP", "TON"} {
			c, err := s.MicroCents(ctx, sym)
			if err != nil {
				t.Errorf("%s/%s: %v", s.Name(), sym, err)
				continue
			}
			t.Logf("%-9s %-4s %d micro-cents", s.Name(), sym, c)
			if c <= 0 {
				t.Errorf("%s/%s answered %d", s.Name(), sym, c)
			}
		}
	}
}

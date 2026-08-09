// Package price answers what ONE WHOLE UNIT of an asset is worth in USD cents.
//
// The rail credits a stablecoin at a FIXED peg and needs none of this. A native
// coin — BTC, XRP, TON — has no peg, so crediting one means taking a market
// price and standing behind it. depositwatch declined to do that for a long
// time, and the reason it gave is still the right reason: "inventing one to
// value a customer's money would be guessing." This package exists so that it
// is not a guess, and it is deliberately small and suspicious.
//
// It never returns a number it cannot justify. Every one of these is a refusal,
// not a fallback:
//
//	fewer than MinSources answered          a single exchange is a single point
//	                                        of failure AND a single point of lie
//	the answers disagree by more than       one source is wrong or stale, and
//	  MaxSpread                             there is no way to tell which
//	a source answered zero or negative      a parse that produced a number by
//	                                        accident
//
// A refusal leaves the deposit UNCREDITED and retried on the next pass. That is
// the safe direction and it costs nothing: the coin is already in custody and
// stays there. The unsafe direction — crediting at a wrong price — converts a
// customer's coin into the wrong number of dollars permanently, and no later
// pass can find it.
//
// WHAT THIS PACKAGE DOES NOT DECIDE, because they are not its business:
//
//	WHEN a deposit is priced. The caller prices it at CONFIRMATION, because that
//	is the moment the money is irreversibly ours; pricing at first-sight would
//	quote a rate for a transfer that can still be orphaned.
//	WHO carries the move between send and confirmation. A customer sends BTC and
//	it confirms an hour later at a different price. Someone absorbs that, and it
//	is a treasury decision rather than an arithmetic one.
package price

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// MinSources is how many INDEPENDENT sources must answer before a quote counts.
//
// Two, not one. One exchange's API is one outage and one bad tick away from
// pricing every deposit in the estate, and nothing downstream could tell a wrong
// number from a right one — the arithmetic is the same either way.
const MinSources = 2

// MaxSpread is how far the sources may disagree, as a fraction of the low quote.
//
// 2% is wide enough that ordinary venue-to-venue drift never blocks a credit
// (BTC, XRP and TON measured 0.002%, 0.018% and 0.13% apart across two venues)
// and tight enough that a stale feed or a fat-finger tick is refused. A spread
// this wide on a liquid pair means one of the two is wrong, and picking the
// "better" one would be the guess this package exists to avoid.
const MaxSpread = 0.02

// ErrNoQuote means no price could be justified. It is deliberately ONE error for
// every refusal above: a caller's only correct response to any of them is to not
// credit and try again, so distinguishing them would invite a caller to handle
// one of them by crediting anyway.
var ErrNoQuote = errors.New("price: no quote could be justified")

// Scale is how many micro-cents make one cent.
//
// A rate is carried in MICRO-CENTS, not cents, and the difference is money. XRP
// trades at $1.04295 and TON at $1.3412: rounded to whole cents those become
// $1.04 and $1.34, which under-credits by 0.28% and 0.09% on EVERY deposit —
// $2.80 lost on a $1,000 XRP top-up, silently, forever. Whole cents are precise
// enough for BTC and nowhere near it for a sub-dollar coin, and a rail cannot
// have a precision that depends on which asset it is pricing.
//
// 10^6 puts the smallest expressible rate at one ten-thousandth of a cent, which
// is finer than any venue quotes, and leaves int64 room to spare: BTC at
// $65,161.49 is 6.5e12 micro-cents against an int64 ceiling of 9.2e18.
const Scale = 1_000_000

// Quote is a justified price, carrying the evidence for it.
//
// Spread and Sources are recorded rather than discarded because a credit made at
// a market rate has to be answerable months later — "what did you value my BTC
// at, and how did you know" is a question a fixed peg never has to answer.
type Quote struct {
	MicroCents int64     // USD cents × Scale, per WHOLE unit
	At         time.Time // when the sources were read
	Spread     float64   // (high-low)/low across the sources that answered
	Sources    []string  // which ones, in the order they are named here
}

// String renders the rate the way an intent records it — plain dollars, for a
// human and for the receipt. It is display only; never compute from it.
func (q Quote) String() string {
	whole := q.MicroCents / (100 * Scale)
	frac := q.MicroCents % (100 * Scale)
	return fmt.Sprintf("%d.%08d", whole, frac)
}

// Source is one venue's opinion of a price. Implementations live beside this
// file and must be INDEPENDENT of one another — two endpoints of the same
// exchange, or two aggregators that both read the same third, are one source
// wearing two names and defeat MinSources entirely.
type Source interface {
	// Name identifies the venue in a Quote and in a refusal.
	Name() string
	// MicroCents returns USD cents × Scale per whole unit of symbol ("BTC",
	// "XRP", "TON"), or an error. It must never return a number it is unsure of.
	MicroCents(ctx context.Context, symbol string) (int64, error)
}

// Oracle combines sources into a quote, or refuses.
type Oracle struct {
	sources []Source
}

// New builds an Oracle over the given sources.
//
// It refuses fewer than MinSources at CONSTRUCTION rather than at the first
// deposit, because a rail configured with one source is a deployment mistake and
// the deposit path is the worst place to discover it.
func New(sources ...Source) (*Oracle, error) {
	if len(sources) < MinSources {
		return nil, fmt.Errorf("price: %d source(s) configured, need at least %d", len(sources), MinSources)
	}
	seen := map[string]bool{}
	for _, s := range sources {
		n := strings.ToLower(strings.TrimSpace(s.Name()))
		if n == "" {
			return nil, errors.New("price: a source with no name cannot be attributed in a quote")
		}
		if seen[n] {
			return nil, fmt.Errorf("price: source %q is configured twice — two names for one venue defeat the independence MinSources is for", s.Name())
		}
		seen[n] = true
	}
	return &Oracle{sources: append([]Source(nil), sources...)}, nil
}

// Quote prices one whole unit of symbol, or refuses.
//
// Sources are read CONCURRENTLY and a slow one cannot hold up the rest beyond
// the caller's context: a deposit scan runs on a timer, and one venue's stall
// must not become the rail's stall.
func (o *Oracle) Quote(ctx context.Context, symbol string) (Quote, error) {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if sym == "" {
		return Quote{}, fmt.Errorf("%w: no symbol given", ErrNoQuote)
	}

	answers := make([]answer, len(o.sources))
	var wg sync.WaitGroup
	for i, s := range o.sources {
		wg.Add(1)
		go func(i int, s Source) {
			defer wg.Done()
			c, err := s.MicroCents(ctx, sym)
			// A source that errors, or answers a non-positive number, simply did
			// not answer. It is never treated as a zero price — that would credit
			// nothing and look like dust rather than like a failure.
			if err != nil || c <= 0 {
				return
			}
			answers[i] = answer{name: s.Name(), micro: c}
		}(i, s)
	}
	wg.Wait()

	var got []answer
	for _, a := range answers {
		if a.micro > 0 {
			got = append(got, a)
		}
	}
	if len(got) < MinSources {
		return Quote{}, fmt.Errorf("%w: %s priced by %d source(s), need %d", ErrNoQuote, sym, len(got), MinSources)
	}

	sort.Slice(got, func(i, j int) bool { return got[i].micro < got[j].micro })
	low, high := got[0].micro, got[len(got)-1].micro
	spread := float64(high-low) / float64(low)
	if spread > MaxSpread {
		names := make([]string, 0, len(got))
		for _, a := range got {
			names = append(names, fmt.Sprintf("%s=%d", a.name, a.micro))
		}
		return Quote{}, fmt.Errorf("%w: %s sources disagree by %.2f%% (max %.2f%%): %s",
			ErrNoQuote, sym, spread*100, MaxSpread*100, strings.Join(names, " "))
	}

	names := make([]string, 0, len(got))
	for _, a := range got {
		names = append(names, a.name)
	}
	// got is already sorted ascending, so the lower middle is one index.
	//
	// The downward bias on an EVEN count is deliberate and matches AmountCents,
	// which truncates a sub-cent remainder DOWN so "the rail can never credit
	// value that was not sent". With two sources — the common case — this is the
	// MINIMUM, so a venue quoting high can never pull a credit above what the
	// conservative venue says the coin was worth. The customer is credited
	// slightly less, never more, and the difference stays on chain in their own
	// deposit rather than accruing to us through a number nobody can check.
	return Quote{
		MicroCents: got[(len(got)-1)/2].micro,
		At:         time.Now().UTC(),
		Spread:     spread,
		Sources:    names,
	}, nil
}

// answer is one source's reply, kept beside its name so a Quote can say who
// priced it and a refusal can say who disagreed.
type answer struct {
	name  string
	micro int64
}

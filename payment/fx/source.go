package fx

import (
	"context"
	"errors"
	"strings"
)

// Lock is how long a quoted rate holds: none is the current rate, no promise and no
// premium; hour and day bind a payment and may carry a premium.
type Lock string

const (
	LockNone Lock = "none"
	LockHour Lock = "hour"
	LockDay  Lock = "day"
)

// Origins of a rate, as a quote names them.
const (
	OriginProcessor = "processor" // a processor's own quote for the payment
	OriginReference = "reference" // the market reference rate
	OriginNone      = "none"      // no conversion: USD, USDC and the like, at 1
)

// Offer is what a Source says one dollar costs in a currency. PerDollar is the rate a
// price is made at (local per dollar, major units); Base is the same before the
// source's own fee and premium, which FeeRate and Premium state as fractions. ID names
// a locked quote to bind a payment to, "" for none; LockExpiresAt is when it lapses
// (unix seconds, 0 for never). Nothing here names a vendor.
type Offer struct {
	ID                string
	Origin            string
	PerDollar         float64
	Base              float64
	FeeRate           float64
	Premium           float64
	Exchange          float64 // the effective rate as USD per unit of the currency
	Reference         float64 // the market reference the source measured against
	ReferenceProvider string
	LockExpiresAt     int64
}

// Source quotes the rate for converting usdCents into cur. A processor's FX quote, the
// market reference rate and the identity rate (a currency that needs no conversion)
// are all Sources, so a price is made one way whichever rail carries the payment.
type Source interface {
	Quote(ctx context.Context, usdCents int64, cur string, lock Lock) (*Offer, error)
}

// Reference is the market reference rate (the ECB's through Frankfurter, a wider table
// where it has no rate). It has no fee and no lock.
type Reference struct{}

func (Reference) Quote(ctx context.Context, _ int64, cur string, _ Lock) (*Offer, error) {
	r, err := Rate(ctx, strings.ToLower(cur))
	if err != nil {
		return nil, err
	}
	return &Offer{Origin: OriginReference, PerDollar: r, Base: r, Exchange: 1 / r}, nil
}

// Identity is the rate of a currency that is not converted: one dollar is a dollar.
type Identity struct{}

func (Identity) Quote(context.Context, int64, string, Lock) (*Offer, error) {
	return &Offer{Origin: OriginNone, PerDollar: 1, Base: 1, Exchange: 1}, nil
}

// Priced is a price made from an Offer: Amount of the currency, in minor units, rounded
// up to Step, at PerDollar. Absorbed says the source's conversion cost is Hanzo's
// (priced on Base, or no fee known), Passed that it is in the price.
type Priced struct {
	Offer     *Offer
	Amount    int64
	PerDollar float64
	Passed    bool
}

// Price makes the price of usdCents in cur from src, falling back to the reference
// rate when src cannot quote (the account has no such quote, the pair is unsupported).
// pass prices on the source's effective rate; otherwise on its base rate. The fallback
// is always absorbed: no fee is guessed. This is the one place a price is made.
func Price(ctx context.Context, src, fallback Source, usdCents int64, cur string, lock Lock, pass bool, step int64) (*Priced, error) {
	if src == nil {
		return nil, errors.New("fx: no rate source")
	}
	if r, err := src.Quote(ctx, usdCents, cur, lock); err == nil {
		rate, passed := r.Base, false
		if pass || r.Origin != OriginProcessor {
			rate, passed = r.PerDollar, pass && r.Origin == OriginProcessor
		}
		if n, cerr := Convert(usdCents, cur, rate, step); cerr == nil {
			return &Priced{Offer: r, Amount: n, PerDollar: rate, Passed: passed}, nil
		}
	}
	if fallback == nil {
		return nil, errors.New("fx: no rate")
	}
	r, err := fallback.Quote(ctx, usdCents, cur, lock)
	if err != nil {
		return nil, err
	}
	n, err := Convert(usdCents, cur, r.PerDollar, step)
	if err != nil {
		return nil, err
	}
	return &Priced{Offer: r, Amount: n, PerDollar: r.PerDollar}, nil
}

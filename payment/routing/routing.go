// Package routing decides which processor takes a NEW card checkout.
//
// The decision is a pure function of a policy and four facts about the buyer, so
// it is the same on every replica and for every asker: the checkout session, the
// public settings read and the admin readout all call Decide.
//
//   - A buyer with a card saved at a processor stays on it. A saved card is bound
//     to the processor that vaulted it; nothing here moves it.
//   - A buyer outside the policy's Square countries goes to Stripe.
//   - A buyer inside them is placed by weight, and placed the same way every time:
//     the bucket is a hash of (org, customer key), so a returning buyer lands where
//     they landed before and raising the weight only ever moves buyers toward Square.
package routing

import (
	"crypto/sha256"
	"encoding/binary"
	"sort"
	"strings"

	"github.com/hanzoai/commerce/payment/processor"
)

// SquareCountries is where Square card processing is enabled: US, CA, AU, JP, GB, IE, FR, ES.
var SquareCountries = []string{"US", "CA", "AU", "JP", "GB", "IE", "FR", "ES"}

// Policy is how an org's new card checkouts are divided.
type Policy struct {
	// SquareWeight is the percentage (0-100) of Square-eligible buyers sent to Square.
	SquareWeight int `json:"squareWeight"`
	// SquareOn and StripeOn take a processor in or out of new checkouts.
	SquareOn bool `json:"squareOn"`
	StripeOn bool `json:"stripeOn"`
	// PayPalOn and CryptoOn take PayPal and the native crypto rail in or out of the
	// options a buyer is shown beside the card slot.
	PayPalOn bool `json:"paypalOn"`
	CryptoOn bool `json:"cryptoOn"`
	// LocalOn lets Stripe's Payment Element offer each country's native methods.
	LocalOn bool `json:"localOn"`
	// Countries is the set Square takes cards in (ISO 3166-1 alpha-2).
	Countries []string `json:"squareCountries"`
}

// Default is the platform policy: half and half across Square's eight countries.
func Default() Policy {
	return Policy{SquareWeight: 50, SquareOn: true, StripeOn: true, PayPalOn: true, CryptoOn: true, LocalOn: true, Countries: append([]string{}, SquareCountries...)}
}

// Normal is p with its countries upper-cased, de-duplicated and sorted, and its
// weight inside 0-100.
func (p Policy) Normal() Policy {
	p.SquareWeight = max(0, min(100, p.SquareWeight))
	seen := map[string]bool{}
	out := []string{}
	for _, c := range p.Countries {
		c = strings.ToUpper(strings.TrimSpace(c))
		if c == "UK" {
			c = "GB"
		}
		if len(c) == 2 && !seen[c] {
			seen[c] = true
			out = append(out, c)
		}
	}
	sort.Strings(out)
	p.Countries = out
	return p
}

// Valid says why p cannot be stored, or "".
func (p Policy) Valid() string {
	if p.SquareWeight < 0 || p.SquareWeight > 100 {
		return "squareWeight must be between 0 and 100"
	}
	for _, c := range p.Countries {
		if n := strings.TrimSpace(c); len(n) != 2 {
			return "squareCountries holds ISO 3166-1 alpha-2 codes"
		}
	}
	if !p.SquareOn && !p.StripeOn {
		return "at least one processor must be on"
	}
	return ""
}

// Input is what a decision is about.
type Input struct {
	// Org and Key make the bucket: Key is the customer's stable id.
	Org, Key string
	// Country is the buyer's billing country, else the edge's geo country; "" if unknown.
	Country string
	// Saved is the processor holding a card the buyer saved; "" for none.
	Saved processor.ProcessorType
}

// Decision is the answer: who takes the checkout, why, and what to mount.
type Decision struct {
	Processor processor.ProcessorType `json:"processor"`
	Reason    string                  `json:"reason"`
	Methods   []string                `json:"methods"`
	Country   string                  `json:"country"`
	// Alternatives are the other ways this buyer may pay, each its own button beside
	// the card slot: "paypal" where PayPal operates, "crypto" where a deposit watcher
	// is running. Filled by the caller, which knows what is configured.
	Alternatives []string `json:"alternatives"`
}

// Bucket is the buyer's place in 0-99, stable per (org, key).
func Bucket(org, key string) int {
	h := sha256.Sum256([]byte(strings.ToLower(org) + "\x00" + strings.ToLower(key)))
	return int(binary.BigEndian.Uint64(h[:8]) % 100)
}

func (p Policy) square(country string) bool {
	country = strings.ToUpper(strings.TrimSpace(country))
	for _, c := range p.Countries {
		if c == country {
			return true
		}
	}
	return false
}

// Decide places one buyer.
func (p Policy) Decide(in Input) Decision {
	country := strings.ToUpper(strings.TrimSpace(in.Country))
	d := Decision{Country: country}
	switch {
	case in.Saved != "":
		d.Processor, d.Reason = in.Saved, "saved"
	case !p.StripeOn:
		d.Processor, d.Reason = processor.Square, "stripe-off"
	case !p.SquareOn:
		d.Processor, d.Reason = processor.Stripe, "square-off"
	case country == "" || !p.square(country):
		d.Processor, d.Reason = processor.Stripe, "country"
	case Bucket(in.Org, in.Key) < p.SquareWeight:
		d.Processor, d.Reason = processor.Square, "split"
	default:
		d.Processor, d.Reason = processor.Stripe, "split"
	}
	d.Methods = p.Methods(d.Processor, country)
	d.Alternatives = []string{}
	if d.Reason == "saved" && d.Processor == processor.Stripe {
		// A card already saved at Stripe is topped up through the saved-card path, not a
		// fresh Payment Element: the plain card form, which also saves it again.
		d.Methods = []string{"card", "apple_pay", "google_pay"}
	}
	return d
}

// Methods are what the checkout mounts for a processor: Square's card form with
// the wallets (and Cash App Pay in the US), or Stripe's Payment Element, which
// offers each country's native methods, or Stripe's plain card form when local
// methods are off.
func (p Policy) Methods(t processor.ProcessorType, country string) []string {
	if t == processor.Square {
		m := []string{"card", "apple_pay", "google_pay"}
		if strings.EqualFold(country, "US") {
			m = append(m, "cash_app")
		}
		return m
	}
	if p.LocalOn {
		return []string{"payment_element"}
	}
	return []string{"card", "apple_pay", "google_pay"}
}

// euro is the euro area plus the other markets Stripe's euro methods (iDEAL,
// Bancontact, SEPA debit, EPS, ...) reach.
var euro = map[string]bool{
	"AT": true, "BE": true, "CY": true, "DE": true, "EE": true, "FI": true, "GR": true, "HR": true,
	"IT": true, "LT": true, "LU": true, "LV": true, "MT": true, "NL": true, "PT": true, "SI": true, "SK": true,
}

// local is the one currency each other market's native methods need.
var local = map[string]string{
	"BR": "brl", // Pix, boleto
	"MX": "mxn", // OXXO
	"PL": "pln", // Przelewy24, BLIK
	"SG": "sgd", // PayNow, GrabPay
	"TH": "thb", // PromptPay
	"MY": "myr", // GrabPay, FPX
	"PH": "php", // GrabPay
}

// LocalCurrency is the currency a buyer in country is shown Stripe's native
// methods in, or "usd" when the country has none that needs its own money (a card,
// Link, a wallet or a buy-now-pay-later method all take dollars). Every currency
// here has two decimals.
func LocalCurrency(country string) string {
	c := strings.ToUpper(strings.TrimSpace(country))
	if euro[c] {
		return "eur"
	}
	if cur, ok := local[c]; ok {
		return cur
	}
	return "usd"
}

// Package routing decides which processor takes a NEW card checkout.
//
// The decision is a pure function of a policy and a few facts, so
// it is the same on every replica and for every asker: the checkout session, the
// public settings read and the admin readout all call Decide.
//
//   - A buyer with a card saved at a processor stays on it. A saved card is bound
//     to the processor that vaulted it; nothing here moves it.
//   - The MERCHANT decides first. Square takes a card only where the merchant's
//     Square location can charge it (a location with card processing, in the
//     checkout's currency); the caller states that as SquareOK. Where it cannot, the checkout is Stripe's, decided before any
//     payment exists. Where the BUYER lives is not a condition: a German buyer paying
//     dollars to a US location is eligible.
//   - An eligible checkout is placed by weight, and placed the same way every time:
//     the bucket is a hash of (org, customer key), so a returning buyer lands where
//     they landed before and raising the weight only ever moves buyers toward Square.
package routing

import (
	"crypto/sha256"
	"encoding/binary"
	"strings"

	"github.com/hanzoai/commerce/payment/processor"
)

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
	// Sticky keeps a payer on the processor the split first placed them on. On by
	// default; off re-evaluates every checkout.
	Sticky bool `json:"sticky"`
	// CurrencyRules send a checkout in a currency to a processor.
	CurrencyRules []CurrencyRule `json:"currencyRules"`
	// CountryRules send buyers in a country to a processor (geographic routing). The
	// buyer's country is a preference here and never a condition of eligibility.
	CountryRules []CountryRule `json:"countryRules"`
	// BrandRules send a card brand (visa, mastercard, ...) to a processor. A brand is
	// known only once the card is entered.
	BrandRules []BrandRule `json:"brandRules"`
	// IssuerCountryRules send cards issued in a country to a processor (for example
	// every card issued outside the US to Stripe). Known only once the card is entered.
	IssuerCountryRules []IssuerRule `json:"issuerCountryRules"`
	// FXSpreadBps is the margin over the mid-market rate when a local-currency payment
	// is quoted, in basis points (0 to 1000). Default 200.
	FXSpreadBps int `json:"fxSpreadBps"`
	// LocalCurrencies sets the currency a buyer in a country is shown local methods in.
	// A country with no entry takes the built-in default; an entry of usd switches
	// local currency off for that country.
	LocalCurrencies []LocalCurrencyRule `json:"localCurrencies"`
	// Fallback is the order of processors tried when the preferred one is ineligible
	// or down, before any payment is started. Empty means the other card processor.
	Fallback []string `json:"fallback"`
	// LocalMethods holds the per-method switches the admin has set. A method with no
	// entry is on, so an empty list is every method on.
	LocalMethods []MethodRule `json:"methods"`
}

// CurrencyRule sends checkouts in Currency (lower-case ISO 4217) to Processor.
type CurrencyRule struct {
	Currency  string `json:"currency"`
	Processor string `json:"processor"`
}

// LocalCurrencyRule sets the currency for buyers in Country.
type LocalCurrencyRule struct {
	Country  string `json:"country"`
	Currency string `json:"currency"`
}

// MethodNote is a plain statement about what a method's switch does, for the admin to label it.
type MethodNote struct {
	Method string `json:"method"`
	// Enforceable says the switch really turns the method off at the processor (or in
	// the list commerce mounts). A switch that cannot be enforced is shown as such.
	Enforceable bool   `json:"enforceable"`
	Note        string `json:"note"`
}

// MethodNotes are the limits of the method switches, as data.
var MethodNotes = []MethodNote{
	{"apple_pay", true, "Stripe cannot exclude Apple Pay by method type, so commerce keeps its own Payment Method Configuration with Apple Pay off and passes it on each intent while this switch is off; it also leaves Apple Pay out of the card and wallet lists commerce mounts."},
	{"google_pay", true, "Stripe cannot exclude Google Pay by method type, so commerce keeps its own Payment Method Configuration with Google Pay off and passes it on each intent while this switch is off; it also leaves Google Pay out of the card and wallet lists commerce mounts."},
	{"cash_app", true, "Cash App Pay is mounted by commerce on a Square location in the United States; this switch removes it from that list."},
}

// CountryRule sends buyers in Country (ISO 3166-1 alpha-2) to Processor.
type CountryRule struct {
	Country   string `json:"country"`
	Processor string `json:"processor"`
}

// BrandRule sends cards of Brand to Processor.
type BrandRule struct {
	Brand     string `json:"brand"`
	Processor string `json:"processor"`
}

// IssuerRule sends cards issued in Country (ISO 3166-1 alpha-2) to Processor.
type IssuerRule struct {
	Country   string `json:"country"`
	Processor string `json:"processor"`
}

// MethodRule switches one payment method on or off.
type MethodRule struct {
	Method string `json:"method"`
	On     bool   `json:"on"`
}

// Brands are the card brands a rule may name.
var Brands = []string{"visa", "mastercard", "amex", "discover", "jcb", "unionpay", "diners"}

// MethodNames are the payment methods an admin may switch. Excludable are the ones
// Stripe's Payment Element can be told to leave out (excluded_payment_method_types);
// apple_pay, google_pay and cash_app are mounted by this service, not by the Element,
// so their switches shape the lists the checkout mounts.
var (
	Excludable  = []string{"pix", "boleto", "oxxo", "ideal", "bancontact", "sepa_debit", "p24", "blik", "eps", "paynow", "promptpay", "grabpay", "alipay", "wechat_pay", "klarna", "affirm", "afterpay_clearpay", "link", "cashapp"}
	MethodNames = append(append([]string{}, Excludable...), "apple_pay", "google_pay", "cash_app")
)

func in(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// MethodCurrencies is the currency each local method can be paid in. A method not
// listed takes any currency the account supports (cards, wallets, Link, BNPL).
var MethodCurrencies = map[string][]string{
	"pix": {"brl"}, "boleto": {"brl"}, "oxxo": {"mxn"}, "ideal": {"eur"}, "bancontact": {"eur"}, "sepa_debit": {"eur"},
	"p24": {"pln", "eur"}, "blik": {"pln"}, "eps": {"eur"}, "paynow": {"sgd"}, "promptpay": {"thb"}, "grabpay": {"sgd", "myr", "php"},
}

// MethodViable says method can be paid in cur ("" or card methods: always).
func MethodViable(method, cur string) bool {
	m, ok := MethodCurrencies[strings.ToLower(method)]
	if !ok {
		return true
	}
	return in(m, strings.ToLower(cur))
}

// KnownBrand and KnownMethod say a name is one an admin may use.
func KnownBrand(b string) bool  { return in(Brands, b) }
func KnownMethod(m string) bool { return in(MethodNames, m) }

// MethodOn says a method is on: it is unless a rule switches it off.
func (p Policy) MethodOn(method string) bool {
	for _, r := range p.LocalMethods {
		if r.Method == method {
			return r.On
		}
	}
	return true
}

// Off lists the Stripe-excludable methods switched off, to leave out of the Payment Element.
func (p Policy) Off() []string {
	out := []string{}
	for _, m := range Excludable {
		if !p.MethodOn(m) {
			out = append(out, m)
		}
	}
	return out
}

// Default is the platform policy: half and half across Square's eight countries.
func Default() Policy {
	return Policy{SquareWeight: 50, SquareOn: true, StripeOn: true, PayPalOn: true, CryptoOn: true, LocalOn: true, Sticky: true, FXSpreadBps: 200, LocalCurrencies: []LocalCurrencyRule{},
		CurrencyRules: []CurrencyRule{}, CountryRules: []CountryRule{}, BrandRules: []BrandRule{}, IssuerCountryRules: []IssuerRule{}, Fallback: []string{}, LocalMethods: []MethodRule{}}
}

// Normal is p with its weight inside 0-100.
func (p Policy) Normal() Policy {
	p.SquareWeight = max(0, min(100, p.SquareWeight))
	cr := []CurrencyRule{}
	for _, r := range p.CurrencyRules {
		cr = append(cr, CurrencyRule{strings.ToLower(strings.TrimSpace(r.Currency)), strings.ToLower(strings.TrimSpace(r.Processor))})
	}
	lc := []LocalCurrencyRule{}
	for _, r := range p.LocalCurrencies {
		lc = append(lc, LocalCurrencyRule{strings.ToUpper(strings.TrimSpace(r.Country)), strings.ToLower(strings.TrimSpace(r.Currency))})
	}
	p.LocalCurrencies = lc
	co := []CountryRule{}
	for _, r := range p.CountryRules {
		co = append(co, CountryRule{strings.ToUpper(strings.TrimSpace(r.Country)), strings.ToLower(strings.TrimSpace(r.Processor))})
	}
	br := []BrandRule{}
	for _, r := range p.BrandRules {
		br = append(br, BrandRule{strings.ToLower(strings.TrimSpace(r.Brand)), strings.ToLower(strings.TrimSpace(r.Processor))})
	}
	ir := []IssuerRule{}
	for _, r := range p.IssuerCountryRules {
		ir = append(ir, IssuerRule{strings.ToUpper(strings.TrimSpace(r.Country)), strings.ToLower(strings.TrimSpace(r.Processor))})
	}
	fb := []string{}
	for _, f := range p.Fallback {
		fb = append(fb, strings.ToLower(strings.TrimSpace(f)))
	}
	mr := []MethodRule{}
	for _, r := range p.LocalMethods {
		mr = append(mr, MethodRule{strings.ToLower(strings.TrimSpace(r.Method)), r.On})
	}
	p.CurrencyRules, p.CountryRules, p.BrandRules, p.IssuerCountryRules, p.Fallback, p.LocalMethods = cr, co, br, ir, fb, mr
	return p
}

// Valid says why p cannot be stored, or "".
func (p Policy) Valid() string {
	if p.SquareWeight < 0 || p.SquareWeight > 100 {
		return "squareWeight must be between 0 and 100"
	}
	if !p.SquareOn && !p.StripeOn {
		return "at least one processor must be on"
	}
	if p.FXSpreadBps < 0 || p.FXSpreadBps > 1000 {
		return "fxSpreadBps must be between 0 and 1000"
	}
	seenLC := map[string]bool{}
	for _, r := range p.LocalCurrencies {
		if !KnownCountry(r.Country) {
			return "localCurrencies: " + strconvQ(r.Country) + " is not an ISO 3166-1 country"
		}
		if len(r.Currency) != 3 {
			return "localCurrencies: " + strconvQ(r.Currency) + " is not an ISO 4217 currency"
		}
		if seenLC[r.Country] {
			return "localCurrencies: " + r.Country + " is named twice"
		}
		seenLC[r.Country] = true
	}
	card := func(proc string) bool { return proc == string(processor.Square) || proc == string(processor.Stripe) }
	seen := map[string]bool{}
	for _, r := range p.CurrencyRules {
		if len(r.Currency) != 3 {
			return "currencyRules: " + strconvQ(r.Currency) + " is not an ISO 4217 currency"
		}
		if !card(r.Processor) {
			return "currencyRules: " + strconvQ(r.Processor) + " is not a card processor (square or stripe)"
		}
		if seen["c"+r.Currency] {
			return "currencyRules: " + r.Currency + " is named twice"
		}
		seen["c"+r.Currency] = true
	}
	for _, r := range p.CountryRules {
		if len(r.Country) != 2 {
			return "countryRules: " + strconvQ(r.Country) + " is not an ISO 3166-1 alpha-2 country"
		}
		if !card(r.Processor) {
			return "countryRules: " + strconvQ(r.Processor) + " is not a card processor (square or stripe)"
		}
		if seen["k"+r.Country] {
			return "countryRules: " + r.Country + " is named twice"
		}
		seen["k"+r.Country] = true
	}
	for _, r := range p.BrandRules {
		if !KnownBrand(r.Brand) {
			return "brandRules: " + strconvQ(r.Brand) + " is not a card brand (" + strings.Join(Brands, ", ") + ")"
		}
		if !card(r.Processor) {
			return "brandRules: " + strconvQ(r.Processor) + " is not a card processor (square or stripe)"
		}
		if seen["b"+r.Brand] {
			return "brandRules: " + r.Brand + " is named twice"
		}
		seen["b"+r.Brand] = true
	}
	for _, r := range p.IssuerCountryRules {
		if len(r.Country) != 2 {
			return "issuerCountryRules: " + strconvQ(r.Country) + " is not an ISO 3166-1 alpha-2 country"
		}
		if !card(r.Processor) {
			return "issuerCountryRules: " + strconvQ(r.Processor) + " is not a card processor (square or stripe)"
		}
		if seen["i"+r.Country] {
			return "issuerCountryRules: " + r.Country + " is named twice"
		}
		seen["i"+r.Country] = true
	}
	for _, f := range p.Fallback {
		if !card(f) {
			return "fallback: " + strconvQ(f) + " is not a card processor (square or stripe)"
		}
		if seen["f"+f] {
			return "fallback: " + f + " is named twice"
		}
		seen["f"+f] = true
	}
	for _, r := range p.LocalMethods {
		if !KnownMethod(r.Method) {
			return "methods: " + strconvQ(r.Method) + " is not a payment method an admin may switch"
		}
		if seen["m"+r.Method] {
			return "methods: " + r.Method + " is named twice"
		}
		seen["m"+r.Method] = true
	}
	return ""
}

func strconvQ(s string) string { return "\"" + s + "\"" }

// Input is what a decision is about.
type Input struct {
	// Org and Key make the bucket: Key is the customer's stable id.
	Org, Key string
	// Country is the buyer's billing country, else the edge's geo country; "" if unknown.
	// It chooses methods (Cash App Pay is for US buyers) and never the processor.
	Country string
	// Assigned is the processor this payer was placed on before. It is a preference:
	// it applies while that processor can take this checkout, and a checkout it cannot
	// take is the other processor's without changing the assignment.
	Assigned processor.ProcessorType
	// SquareOK says a Square location the merchant holds can take this checkout's
	// card in its currency (square.Provider.Eligible).
	SquareOK bool
	// SquareWhy says why Square is not eligible, for the reason chain.
	SquareWhy string
	// Saved is the processor holding a card the buyer saved; "" for none.
	Saved processor.ProcessorType
	// Currency is what the checkout is paid in; the currency rules read it.
	Currency string
	// Method is how the buyer chose to pay when they have: cash_app is Square's, a
	// local method (pix, ideal, ...) is Stripe's. Empty or card decides nothing.
	Method string
	// Brand and IssuerCountry describe the card once it is entered; empty before.
	Brand, IssuerCountry string
}

// Decision is the answer: who takes the checkout, why, and what to mount.
type Decision struct {
	Processor processor.ProcessorType `json:"processor"`
	Reason    string                  `json:"reason"`
	Methods   []string                `json:"methods"`
	Country   string                  `json:"country"`
	// Rule names the rule that decided: saved, merchant, currency, method, issuer,
	// brand, assigned, split, toggle or fallback.
	Rule string `json:"rule"`
	// Chain is the seven steps in the order they are taken, each applied or skipped and
	// why: saved, eligibility, method, overrides, sticky, weight, fallback.
	Chain []Step `json:"chain"`
	// LocalCurrency and FXSpreadBps are what a local-method payment for this buyer
	// would be quoted in and at: the policy's, shown so the admin sees them applied.
	LocalCurrency string `json:"localCurrency"`
	FXSpreadBps   int    `json:"fxSpreadBps"`
	// Version is the policy version this decision was made under.
	Version int `json:"version"`
	// Quote is the price of a local-currency payment for the amount the caller asked
	// about, when it asked and the checkout is Stripe's.
	Quote *Quote `json:"quote,omitempty"`
	// Alternatives are the other ways this buyer may pay, each its own button beside
	// the card slot: "paypal" where PayPal operates, "crypto" where a deposit watcher
	// is running. Filled by the caller, which knows what is configured.
	Alternatives []string `json:"alternatives"`
}

// Quote is what a local-currency payment will cost, disclosed before the buyer
// authorizes: Amount of Currency buys UsdCents of credit, at Rate per dollar, which
// carries MarkupBps over the mid-market rate. The amount charged at confirm is Amount.
type Quote struct {
	Amount    int64   `json:"amount"`
	Currency  string  `json:"currency"`
	UsdCents  int64   `json:"usdCents"`
	Rate      float64 `json:"rate"`
	MarkupBps int     `json:"markupBps"`
}

// Step is one step of the reason chain.
type Step struct {
	Step    string `json:"step"`
	Applied bool   `json:"applied"`
	Why     string `json:"why"`
}

// Bucket is the buyer's place in 0-99, stable per (org, key).
func Bucket(org, key string) int {
	h := sha256.Sum256([]byte(strings.ToLower(org) + "\x00" + strings.ToLower(key)))
	return int(binary.BigEndian.Uint64(h[:8]) % 100)
}

// Usable says processor t can take this checkout at all: switched on, and for Square
// only where the merchant's location can charge it.
func (p Policy) Usable(t processor.ProcessorType, in Input) bool {
	switch t {
	case processor.Square:
		return p.SquareOn && in.SquareOK
	case processor.Stripe:
		return p.StripeOn
	}
	return false
}

func (p Policy) ruled(proc string, in Input) (processor.ProcessorType, bool) {
	t := processor.ProcessorType(proc)
	return t, p.Usable(t, in)
}

// Decide places one buyer, and says why step by step. The order is the policy and
// it is fixed: (1) a saved card stays where it was saved; (2) the merchant: a processor
// that cannot take the charge is out of every later step; (3) the method the buyer
// chose; (4) the admin's overrides: currency, buyer country, then card issuing country
// and brand where that metadata is known; (5) the payer's earlier placement when
// stickiness is on; (6) the split weight; (7) the fallback, which the caller applies
// only before a payment exists.
func (p Policy) Decide(in Input) Decision {
	country := strings.ToUpper(strings.TrimSpace(in.Country))
	cur := strings.ToLower(strings.TrimSpace(in.Currency))
	method := strings.ToLower(strings.TrimSpace(in.Method))
	issuer := strings.ToUpper(strings.TrimSpace(in.IssuerCountry))
	brand := strings.ToLower(strings.TrimSpace(in.Brand))
	d := Decision{Country: country}
	decided := false
	step := func(name string, applied bool, why string) {
		d.Chain = append(d.Chain, Step{name, applied, why})
	}
	set := func(t processor.ProcessorType, reason, name string) {
		d.Processor, d.Reason, d.Rule, decided = t, reason, name, true
	}

	// 1. saved
	if in.Saved != "" {
		set(in.Saved, "saved", "saved")
		step("saved", true, "a card is saved at "+string(in.Saved)+", so the charge stays there whatever the policy says")
	} else {
		step("saved", false, "no saved method")
	}

	// 2. eligibility
	var out []string
	if !p.SquareOn {
		out = append(out, "square is switched off")
	} else if !in.SquareOK {
		why := in.SquareWhy
		if why == "" {
			why = "no Square merchant location takes cards in " + cur
		}
		out = append(out, "square excluded: "+why)
	}
	if !p.StripeOn {
		out = append(out, "stripe is switched off")
	}
	if len(out) > 0 {
		step("eligibility", true, strings.Join(out, "; "))
	} else {
		step("eligibility", false, "every processor that is on can take this charge")
	}

	// 3. explicit method
	if !decided && method != "" && method != "card" {
		want := processor.Stripe
		if method == "cash_app" {
			want = processor.Square
		}
		if p.Usable(want, in) {
			set(want, "method", "method")
			step("method", true, "the buyer chose "+method+", which "+string(want)+" takes")
		} else {
			step("method", false, "the buyer chose "+method+" but "+string(want)+" cannot take it here")
		}
	} else if decided {
		step("method", false, "skipped: already decided")
	} else {
		step("method", false, "the buyer has chosen no particular method")
	}

	// 4. overrides
	if decided {
		step("overrides", false, "skipped: already decided")
	} else {
		var why []string
		try := func(kind, key, proc, rule string) bool {
			t, ok := p.ruled(proc, in)
			if !ok {
				why = append(why, kind+" "+key+" -> "+proc+" ignored: "+proc+" cannot take this charge")
				return false
			}
			set(t, rule, rule)
			why = append(why, kind+" "+key+" -> "+proc)
			return true
		}
		if cur != "" {
			for _, r := range p.CurrencyRules {
				if r.Currency == cur && try("currency", cur, r.Processor, "currency") {
					break
				}
			}
		}
		if !decided && country != "" {
			for _, r := range p.CountryRules {
				if r.Country == country && try("buyer country", country, r.Processor, "country") {
					break
				}
			}
		}
		if !decided && issuer != "" {
			for _, r := range p.IssuerCountryRules {
				if r.Country == issuer && try("card issued in", issuer, r.Processor, "issuer") {
					break
				}
			}
		}
		if !decided && brand != "" {
			for _, r := range p.BrandRules {
				if r.Brand == brand && try("card brand", brand, r.Processor, "brand") {
					break
				}
			}
		}
		cardRules := len(p.IssuerCountryRules)+len(p.BrandRules) > 0
		if cardRules && issuer == "" && brand == "" {
			why = append(why, "card rules: not applicable: card metadata unknown before selection")
		}
		switch {
		case decided:
			step("overrides", true, strings.Join(why, "; "))
		case len(why) > 0:
			step("overrides", false, strings.Join(why, "; "))
		default:
			step("overrides", false, "no rule matches")
		}
	}

	// 5. sticky
	switch {
	case decided:
		step("sticky", false, "skipped: already decided")
	case !p.Sticky:
		step("sticky", false, "customer affinity is off")
	case in.Assigned == "":
		step("sticky", false, "this payer has not been placed yet")
	case p.Usable(in.Assigned, in):
		set(in.Assigned, "assigned", "assigned")
		step("sticky", true, "placed on "+string(in.Assigned)+" before, and it can take this charge")
	default:
		step("sticky", false, "placed on "+string(in.Assigned)+" before, but it cannot take this charge now; the placement is kept for the next eligible checkout")
	}

	// 6. weight
	if decided {
		step("weight", false, "skipped: already decided")
	} else {
		switch {
		case !p.StripeOn:
			set(processor.Square, "stripe-off", "toggle")
			step("weight", true, "stripe is off, so square")
		case !p.SquareOn:
			set(processor.Stripe, "square-off", "toggle")
			step("weight", true, "square is off, so stripe")
		case !in.SquareOK:
			set(processor.Stripe, "merchant", "merchant")
			step("weight", true, "square cannot take this charge, so stripe")
		case Bucket(in.Org, in.Key) < p.SquareWeight:
			set(processor.Square, "split", "split")
			step("weight", true, "bucket below the square weight of "+strconvI(p.SquareWeight))
		default:
			set(processor.Stripe, "split", "split")
			step("weight", true, "bucket at or above the square weight of "+strconvI(p.SquareWeight))
		}
	}

	// 7. fallback is the caller's; recorded here as not needed until it applies.
	step("fallback", false, "not needed: the chosen processor can take the charge")

	d.Methods = p.Methods(d.Processor, country)
	d.Alternatives = []string{}
	if d.Reason == "saved" && d.Processor == processor.Stripe {
		// A card already saved at Stripe is topped up through the saved-card path, not a
		// fresh Payment Element: the plain card form, which also saves it again.
		d.Methods = p.wallets([]string{"card", "apple_pay", "google_pay"})
	}
	return d
}

func strconvI(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}

// wallets drops the wallet entries an admin has switched off.
func (p Policy) wallets(m []string) []string {
	out := make([]string, 0, len(m))
	for _, x := range m {
		if (x == "apple_pay" || x == "google_pay" || x == "cash_app") && !p.MethodOn(x) {
			continue
		}
		out = append(out, x)
	}
	return out
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
		return p.wallets(m)
	}
	if p.LocalOn {
		return []string{"payment_element"}
	}
	return p.wallets([]string{"card", "apple_pay", "google_pay"})
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

// LocalCurrency is the currency a buyer in country is shown local methods in: the
// policy's entry for the country, else the built-in default, or "usd" when the country
// has none that needs its own money (a card, Link, a wallet or a buy-now-pay-later
// method all take dollars). Every currency here has two decimals.
func (p Policy) LocalCurrency(country string) string {
	c := strings.ToUpper(strings.TrimSpace(country))
	for _, r := range p.LocalCurrencies {
		if r.Country == c {
			return r.Currency
		}
	}
	return DefaultLocalCurrency(c)
}

// DefaultLocalCurrency is the built-in table a country with no policy entry takes.
func DefaultLocalCurrency(country string) string {
	c := strings.ToUpper(strings.TrimSpace(country))
	if euro[c] {
		return "eur"
	}
	if cur, ok := local[c]; ok {
		return cur
	}
	return "usd"
}

// KnownCountry says code is an ISO 3166-1 alpha-2 country.
func KnownCountry(code string) bool {
	return strings.Contains(" "+isoCountries+" ", " "+strings.ToUpper(code)+" ") && len(code) == 2
}

const isoCountries = "AD AE AF AG AI AL AM AO AQ AR AS AT AU AW AX AZ BA BB BD BE BF BG BH BI BJ BL BM BN BO BQ BR BS BT BV BW BY BZ CA CC CD CF CG CH CI CK CL CM CN CO CR CU CV CW CX CY CZ DE DJ DK DM DO DZ EC EE EG EH ER ES ET FI FJ FK FM FO FR GA GB GD GE GF GG GH GI GL GM GN GP GQ GR GS GT GU GW GY HK HM HN HR HT HU ID IE IL IM IN IO IQ IR IS IT JE JM JO JP KE KG KH KI KM KN KP KR KW KY KZ LA LB LC LI LK LR LS LT LU LV LY MA MC MD ME MF MG MH MK ML MM MN MO MP MQ MR MS MT MU MV MW MX MY MZ NA NC NE NF NG NI NL NO NP NR NU NZ OM PA PE PF PG PH PK PL PM PN PR PS PT PW PY QA RE RO RS RU RW SA SB SC SD SE SG SH SI SJ SK SL SM SN SO SR SS ST SV SX SY SZ TC TD TF TG TH TJ TK TL TM TN TO TR TT TV TW TZ UA UG UM US UY UZ VA VC VE VG VI VN VU WF WS YE YT ZA ZM ZW"

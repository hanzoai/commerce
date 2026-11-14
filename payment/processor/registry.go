package processor

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/hanzoai/commerce/models/types/currency"
)

// Registry manages available payment processors
type Registry struct {
	mu         sync.RWMutex
	processors map[ProcessorType]PaymentProcessor
	config     *RegistryConfig
}

// RegistryConfig holds configuration for processor selection.
//
// Fiat selection has exactly ONE mechanism: ProcessorPriority gives the
// preference order, DisabledProcessors is the deny policy, and Fiat walks the
// order for the first processor that can take the payment. Crypto selection
// keeps a single preferred processor (DefaultCryptoProcessor) plus a
// currency-driven fallback order, because crypto routing is by currency, not
// preference order.
type RegistryConfig struct {
	// DefaultCryptoProcessor is the preferred processor for crypto currencies.
	DefaultCryptoProcessor ProcessorType

	// ProcessorPriority is the fiat preference order (first that can take the
	// payment wins). Square heads it and Stripe follows it, which is the whole of
	// "Stripe wherever Square does not work": see Fiat.
	ProcessorPriority []ProcessorType

	// DisabledProcessors is the deny set: a processor in it is NEVER returned by
	// SelectProcessor, Fiat or Get, from any branch. DefaultConfig fills it from
	// the deployment's charge policy (the routing policy): every type it does not
	// name.
	DisabledProcessors map[ProcessorType]bool
}

// DefaultConfig returns the default registry configuration.
//
// Fiat: Square first, Stripe second, then the rest, and Fiat takes the first
// that can serve the payment. WHICH processors may take a NEW charge at all is
// the deployment's policy (the routing policy), which production sets to Stripe
// alone explicitly.
//
// Crypto: DefaultCryptoProcessor (MPC) plus the crypto fallback order in
// SelectProcessor.
func DefaultConfig() *RegistryConfig {
	return &RegistryConfig{
		DefaultCryptoProcessor: MPC,
		ProcessorPriority: []ProcessorType{
			Square, // takes every payment it can, where a deployment allows it
			Stripe, // takes what Square cannot, or everything when it is alone
			Adyen,
			PayPal,
			Braintree,
			Recurly,
			LemonSqueezy,
			// crypto
			Kraken,
			MPC,
			SolanaPay,
			CoinbaseCommerce,
			OpenNode,
			MoonPay,
			Ethereum,
			Bitcoin,
			Wire,
		},
		DisabledProcessors: map[ProcessorType]bool{},
	}
}

// ChargePolicy answers which processors the deployment lets take NEW charges: the
// processors the routing policy lists as on. It is installed once, at boot, by the host
// that holds the policy (api/billing), and asked at CALL time, so an admin's change
// applies to the next attempt with no restart. A customer's card already on file keeps
// being charged by its processor (Registry.Existing), whatever it says.
//
// With none installed (a library user, a test) every processor is allowed.
type ChargePolicy func() map[ProcessorType]bool

var chargePolicy atomic.Pointer[ChargePolicy]

// SetChargePolicy installs the policy and returns what restores the previous one.
func SetChargePolicy(f ChargePolicy) (restore func()) {
	old := chargePolicy.Load()
	if f == nil {
		chargePolicy.Store(nil)
	} else {
		chargePolicy.Store(&f)
	}
	return func() { chargePolicy.Store(old) }
}

// ChargeNames is the policy a list of names states: "*" is every processor.
func ChargeNames(names ...string) map[ProcessorType]bool {
	allowed := map[ProcessorType]bool{}
	for _, list := range names {
		for _, name := range strings.Split(list, ",") {
			switch name = strings.ToLower(strings.TrimSpace(name)); name {
			case "":
			case "*":
				for _, t := range allTypes {
					allowed[t] = true
				}
			default:
				allowed[ProcessorType(name)] = true
			}
		}
	}
	return allowed
}

// every processor type that can be named by the policy.
var allTypes = []ProcessorType{
	Stripe, Square, PayPal, Adyen, Braintree, Recurly, LemonSqueezy, BitPay,
	CoinbaseCommerce, OpenNode, Bitcoin, Ethereum, MPC, Wire, SolanaPay, MoonPay, Kraken,
}

// chargeAllowed is the set of processor types the installed policy allows.
func chargeAllowed() map[ProcessorType]bool {
	if f := chargePolicy.Load(); f != nil {
		return (*f)()
	}
	return ChargeNames("*")
}

// disabledProcessors is every processor type the policy does not allow.
func disabledProcessors() map[ProcessorType]bool {
	allowed := chargeAllowed()
	disabled := make(map[ProcessorType]bool)
	for _, t := range allTypes {
		if !allowed[t] {
			disabled[t] = true
		}
	}
	return disabled
}

// DisabledByPolicy reports whether the charge policy keeps processor t from taking a
// NEW charge. It is the SINGLE source of truth for "is this processor off for new
// charges," shared by the registry AND by surfaces that list processors without one.
func DisabledByPolicy(t ProcessorType) bool {
	return disabledProcessors()[t]
}

// off says t may not take a new charge now: denied in this registry's own set, or by the
// policy as it stands at this call.
func (r *Registry) off(t ProcessorType) bool {
	return r.config.DisabledProcessors[t] || DisabledByPolicy(t)
}

var (
	globalRegistry *Registry
	once           sync.Once
)

// Global returns the global registry instance
func Global() *Registry {
	once.Do(func() {
		globalRegistry = NewRegistry(DefaultConfig())
	})
	return globalRegistry
}

// NewRegistry creates a new processor registry
func NewRegistry(config *RegistryConfig) *Registry {
	if config == nil {
		config = DefaultConfig()
	}
	return &Registry{
		processors: make(map[ProcessorType]PaymentProcessor),
		config:     config,
	}
}

// Register adds a processor to the registry
func (r *Registry) Register(p PaymentProcessor) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.processors[p.Type()] = p
}

// Unregister removes a processor from the registry
func (r *Registry) Unregister(t ProcessorType) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.processors, t)
}

// Get retrieves a processor by type
func (r *Registry) Get(t ProcessorType) (PaymentProcessor, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.off(t) {
		return nil, fmt.Errorf("processor %s is disabled", t)
	}

	p, ok := r.processors[t]
	if !ok {
		return nil, fmt.Errorf("processor %s not registered", t)
	}
	return p, nil
}

// Existing returns the configured processor t for a customer already on it —
// a card on file with t, a payment t took — IGNORING the new-charge policy: the
// policy decides where NEW money goes, not whether a subscription renews on the
// card it was bought with.
func (r *Registry) Existing(ctx context.Context, t ProcessorType) (PaymentProcessor, error) {
	p, err := r.Registered(t)
	if err != nil {
		return nil, err
	}
	if !p.IsAvailable(ctx) {
		return nil, fmt.Errorf("processor %s is not available", t)
	}
	return p, nil
}

// Registered returns a processor by type IGNORING the disabled-processor policy.
//
// Use this only for INBOUND paths — webhook validation, admin/historical
// operations — that must reach a registered processor even when it is disabled
// for new-charge SELECTION (e.g. Stripe, which stays able to validate existing
// customers' webhooks). Outbound charge selection must use Get / SelectProcessor,
// which honor the deny policy.
func (r *Registry) Registered(t ProcessorType) (PaymentProcessor, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	p, ok := r.processors[t]
	if !ok {
		return nil, fmt.Errorf("processor %s not registered", t)
	}
	return p, nil
}

// GetCrypto retrieves a crypto processor
func (r *Registry) GetCrypto(t ProcessorType) (CryptoProcessor, error) {
	p, err := r.Get(t)
	if err != nil {
		return nil, err
	}

	cp, ok := p.(CryptoProcessor)
	if !ok {
		return nil, fmt.Errorf("processor %s does not support crypto operations", t)
	}
	return cp, nil
}

// GetSubscription retrieves a subscription processor
func (r *Registry) GetSubscription(t ProcessorType) (SubscriptionProcessor, error) {
	p, err := r.Get(t)
	if err != nil {
		return nil, err
	}

	sp, ok := p.(SubscriptionProcessor)
	if !ok {
		return nil, fmt.Errorf("processor %s does not support subscriptions", t)
	}
	return sp, nil
}

// GetCustomer retrieves a customer processor
func (r *Registry) GetCustomer(t ProcessorType) (CustomerProcessor, error) {
	p, err := r.Get(t)
	if err != nil {
		return nil, err
	}

	cp, ok := p.(CustomerProcessor)
	if !ok {
		return nil, fmt.Errorf("processor %s does not support customer management", t)
	}
	return cp, nil
}

// Need is one fiat payment as the selection rule sees it.
type Need struct {
	// Currency is what the payment is made in.
	Currency currency.Type
	// Country is the buyer's ISO 3166-1 alpha-2 code; "" when it is not known,
	// which no processor is refused for.
	Country string
	// Method is how the buyer pays: "card", "apple_pay", "google_pay",
	// "cash_app", "ach", ...; "" when the token is already minted and its method
	// is the minting processor's business.
	Method string
	// Want is the processor the caller named; "" for none.
	Want ProcessorType
}

// Reach is implemented by a processor that takes payments from buyers in some
// countries, or by some methods, only. A processor that is not one is matched
// only by a Need that names neither (a token already minted), because nothing
// says which countries or methods it takes.
type Reach interface {
	Reaches(country, method string) bool
}

// Merchant is implemented by a processor whose ability to take a currency is a
// fact about the merchant's own account, read from the processor (a Square location
// in one currency, in one country), not a list kept here. Eligible answers the
// country of the account that can take a card in cur, and false when none can.
type Merchant interface {
	Eligible(ctx context.Context, cur currency.Type) (country string, ok bool)
}

// Fiat names the processor that takes a fiat payment.
//
// A payment the caller names a processor for goes to that processor or to none:
// a token belongs to the processor that minted it, so substituting another one
// is a charge that cannot succeed. Otherwise the first processor in priority
// order that can take the payment takes it — Square wherever Square can, and
// Stripe, next in the order, wherever Square cannot.
//
// "Can take it" is the same test for every processor (canTake): registered,
// not denied, configured, in the currency, and, for a processor with a Reach,
// in the buyer's country and by the buyer's method.
func (r *Registry) Fiat(ctx context.Context, n Need) (PaymentProcessor, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.fiat(ctx, n)
}

func (r *Registry) fiat(ctx context.Context, n Need) (PaymentProcessor, error) {
	if n.Want != "" {
		if p := r.canTake(ctx, n.Want, n); p != nil {
			return p, nil
		}
		return nil, fmt.Errorf("processor %s cannot take a %s payment here", n.Want, n.Currency)
	}
	for _, t := range r.config.ProcessorPriority {
		if p := r.canTake(ctx, t, n); p != nil {
			return p, nil
		}
	}
	return nil, fmt.Errorf("no processor available for currency %s", n.Currency)
}

// Sanctioned reports whether code (ISO 3166-1 alpha-2) names a jurisdiction no
// fiat processor here may take a payment from: the comprehensively sanctioned
// countries (Cuba, Iran, North Korea, Syria) plus Russia and Belarus.
func Sanctioned(code string) bool {
	switch strings.ToUpper(strings.TrimSpace(code)) {
	case "CU", "IR", "KP", "SY", "RU", "BY":
		return true
	}
	return false
}

// canTake returns processor t when it can take the payment n, else nil.
func (r *Registry) canTake(ctx context.Context, t ProcessorType, n Need) PaymentProcessor {
	if r.off(t) || Sanctioned(n.Country) {
		return nil
	}
	p, ok := r.processors[t]
	if !ok || !p.IsAvailable(ctx) || !supportssCurrency(p, n.Currency) {
		return nil
	}
	if rp, ok := p.(Reach); ok {
		if !rp.Reaches(n.Country, n.Method) {
			return nil
		}
	} else if n.Country != "" || n.Method != "" {
		return nil
	}
	return p
}

// Cards is the card form each currency is paid through for a buyer in country
// (ISO 3166-1 alpha-2, "" when unknown): Fiat's answer for a card, for every
// currency some card processor here takes. A currency no processor takes by
// card is absent. It is what a checkout reads to mount Square's card form or
// Stripe's, so the browser tokenizes with the processor the charge will use.
func (r *Registry) Cards(ctx context.Context, country string) map[currency.Type]ProcessorType {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := map[currency.Type]ProcessorType{}
	for _, t := range r.config.ProcessorPriority {
		p, ok := r.processors[t]
		if !ok {
			continue
		}
		for _, c := range p.SupportedCurrencies() {
			if _, done := out[c]; done {
				continue
			}
			if chosen, err := r.fiat(ctx, Need{Currency: c, Country: country, Method: "card"}); err == nil {
				out[c] = chosen.Type()
			}
		}
	}
	return out
}

// SelectProcessor chooses the processor for a payment whose token is already
// minted: the one named in Options["processor"], a crypto processor for a
// crypto currency, else Fiat.
func (r *Registry) SelectProcessor(ctx context.Context, req PaymentRequest) (PaymentProcessor, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// An explicit preference is a Want: that processor or none.
	var want ProcessorType
	switch pref := req.Options["processor"].(type) {
	case ProcessorType:
		want = pref
	case string:
		want = ProcessorType(strings.ToLower(strings.TrimSpace(pref)))
	}
	if want != "" {
		return r.fiat(ctx, Need{Currency: req.Currency, Want: want})
	}

	// For crypto currencies, use crypto processor
	if IsCryptoCurrency(req.Currency) {
		if p, ok := r.processors[r.config.DefaultCryptoProcessor]; ok {
			if !r.off(r.config.DefaultCryptoProcessor) && p.IsAvailable(ctx) {
				return p, nil
			}
		}
		// Fallback to any available crypto processor
		for _, t := range []ProcessorType{Kraken, SolanaPay, MPC, Ethereum, Bitcoin} {
			if p, ok := r.processors[t]; ok {
				if !r.off(t) && p.IsAvailable(ctx) {
					if supportssCurrency(p, req.Currency) {
						return p, nil
					}
				}
			}
		}
	}

	return r.fiat(ctx, Need{Currency: req.Currency})
}

// SelectSubscriptionProcessor chooses a processor that supports subscriptions
func (r *Registry) SelectSubscriptionProcessor(ctx context.Context, req PaymentRequest) (SubscriptionProcessor, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Try priority order for subscription-capable processors
	for _, t := range r.config.ProcessorPriority {
		if r.off(t) {
			continue
		}
		if p, ok := r.processors[t]; ok {
			if sp, ok := p.(SubscriptionProcessor); ok {
				if p.IsAvailable(ctx) && supportssCurrency(p, req.Currency) {
					return sp, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("no subscription processor available for currency %s", req.Currency)
}

// Available returns all available processors
func (r *Registry) Available(ctx context.Context) []PaymentProcessor {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]PaymentProcessor, 0, len(r.processors))
	for t, p := range r.processors {
		if !r.off(t) && p.IsAvailable(ctx) {
			result = append(result, p)
		}
	}
	return result
}

// RegisteredProcessors returns ALL registered processors IGNORING the
// disabled-processor policy. For inbound webhook validation: a processor that is
// disabled for new-charge selection (e.g. Stripe) must still be reachable to
// validate its existing customers' webhook deliveries. Callers filter by
// IsAvailable themselves. Outbound selection must use Available / SelectProcessor.
func (r *Registry) RegisteredProcessors() []PaymentProcessor {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]PaymentProcessor, 0, len(r.processors))
	for _, p := range r.processors {
		result = append(result, p)
	}
	return result
}

// ListTypes returns all registered processor types
func (r *Registry) ListTypes() []ProcessorType {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]ProcessorType, 0, len(r.processors))
	for t := range r.processors {
		result = append(result, t)
	}
	return result
}

// SetConfig updates the registry configuration
func (r *Registry) SetConfig(config *RegistryConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.config = config
}

// DisableProcessor disables a processor
func (r *Registry) DisableProcessor(t ProcessorType) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.config.DisabledProcessors[t] = true
}

// EnableProcessor enables a processor
func (r *Registry) EnableProcessor(t ProcessorType) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.config.DisabledProcessors, t)
}

// supportssCurrency checks if a processor supports a currency
func supportssCurrency(p PaymentProcessor, c currency.Type) bool {
	for _, supported := range p.SupportedCurrencies() {
		if supported == c {
			return true
		}
	}
	return false
}

// Package-level convenience functions

// Register adds a processor to the global registry
func Register(p PaymentProcessor) {
	Global().Register(p)
}

// Get retrieves a processor from the global registry
func Get(t ProcessorType) (PaymentProcessor, error) {
	return Global().Get(t)
}

// SelectProcessor selects a processor from the global registry
func SelectProcessor(ctx context.Context, req PaymentRequest) (PaymentProcessor, error) {
	return Global().SelectProcessor(ctx, req)
}

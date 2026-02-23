package processor

import (
	"context"
	"fmt"
	"sync"

	"github.com/hanzoai/commerce/models/types/currency"
)

// Registry manages available payment processors
type Registry struct {
	mu         sync.RWMutex
	processors map[ProcessorType]PaymentProcessor
	config     *RegistryConfig
}

// RegistryConfig holds configuration for processor selection
type RegistryConfig struct {
	// DefaultFiatProcessor is the default for fiat currencies
	DefaultFiatProcessor ProcessorType

	// DefaultCryptoProcessor is the default for crypto currencies
	DefaultCryptoProcessor ProcessorType

	// ProcessorPriority defines the order to try processors
	ProcessorPriority []ProcessorType

	// DisabledProcessors lists processors that should not be used
	DisabledProcessors map[ProcessorType]bool
}

// DefaultConfig returns the default registry configuration
func DefaultConfig() *RegistryConfig {
	return &RegistryConfig{
		DefaultFiatProcessor:   Square,
		DefaultCryptoProcessor: MPC,
		ProcessorPriority: []ProcessorType{
			Stripe,
			Square,
			Adyen,
			PayPal,
			Braintree,
			Recurly,
			LemonSqueezy,
			MPC,
			Ethereum,
			Bitcoin,
		},
		DisabledProcessors: make(map[ProcessorType]bool),
	}
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

	if r.config.DisabledProcessors[t] {
		return nil, fmt.Errorf("processor %s is disabled", t)
	}

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

// SelectProcessor chooses the best processor for the payment
func (r *Registry) SelectProcessor(ctx context.Context, req PaymentRequest) (PaymentProcessor, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Check for explicit preference
	if pref, ok := req.Options["processor"].(ProcessorType); ok {
		if p, ok := r.processors[pref]; ok {
			if !r.config.DisabledProcessors[pref] && p.IsAvailable(ctx) {
				return p, nil
			}
		}
	}

	// Check for string preference
	if prefStr, ok := req.Options["processor"].(string); ok {
		pref := ProcessorType(prefStr)
		if p, ok := r.processors[pref]; ok {
			if !r.config.DisabledProcessors[pref] && p.IsAvailable(ctx) {
				return p, nil
			}
		}
	}

	// For crypto currencies, use crypto processor
	if IsCryptoCurrency(req.Currency) {
		if p, ok := r.processors[r.config.DefaultCryptoProcessor]; ok {
			if !r.config.DisabledProcessors[r.config.DefaultCryptoProcessor] && p.IsAvailable(ctx) {
				return p, nil
			}
		}
		// Fallback to any available crypto processor
		for _, t := range []ProcessorType{MPC, Ethereum, Bitcoin} {
			if p, ok := r.processors[t]; ok {
				if !r.config.DisabledProcessors[t] && p.IsAvailable(ctx) {
					if supportssCurrency(p, req.Currency) {
						return p, nil
					}
				}
			}
		}
	}

	// For fiat, use priority order
	for _, t := range r.config.ProcessorPriority {
		if r.config.DisabledProcessors[t] {
			continue
		}
		if p, ok := r.processors[t]; ok {
			if p.IsAvailable(ctx) && supportssCurrency(p, req.Currency) {
				return p, nil
			}
		}
	}

	return nil, fmt.Errorf("no processor available for currency %s", req.Currency)
}

// SelectSubscriptionProcessor chooses a processor that supports subscriptions
func (r *Registry) SelectSubscriptionProcessor(ctx context.Context, req PaymentRequest) (SubscriptionProcessor, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Try priority order for subscription-capable processors
	for _, t := range r.config.ProcessorPriority {
		if r.config.DisabledProcessors[t] {
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
		if !r.config.DisabledProcessors[t] && p.IsAvailable(ctx) {
			result = append(result, p)
		}
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

// Package providers is a convenience package that imports all payment provider
// packages, ensuring they are available for the processor registry. Import this
// package with a blank identifier in your main application to register all
// providers:
//
//	import _ "github.com/hanzoai/commerce/payment/providers"
package providers

import (
	// Import all provider packages so their init() functions register
	// with the global processor registry if they use init-based registration.
	// For providers that require configuration, use their NewProcessor()
	// constructors directly.
	_ "github.com/hanzoai/commerce/payment/providers/adyen"
	_ "github.com/hanzoai/commerce/payment/providers/braintree"
	_ "github.com/hanzoai/commerce/payment/providers/lemonsqueezy"
	_ "github.com/hanzoai/commerce/payment/providers/paypal"
	_ "github.com/hanzoai/commerce/payment/providers/recurly"
)

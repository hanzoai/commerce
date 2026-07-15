package billing

import (
	"github.com/zap-proto/zip"
)

// GetCapabilities returns the billing platform's supported features,
// payment methods, and currencies.
//
//	GET /v1/billing/capabilities
func GetCapabilities(c *zip.Ctx) error {
	return c.JSON(200, map[string]any{
		"paymentMethods": []string{
			"card",
			"bank_account",
			"balance",
			"crypto",
		},
		"currencies": []string{
			"usd", "eur", "gbp", "cad", "aud", "jpy", "chf",
			"btc", "eth", "sol", "usdc", "usdt", "lux",
		},
		"features": []string{
			"subscriptions",
			"subscription_items",
			"metered_billing",
			"invoicing",
			"credit_grants",
			"credit_notes",
			"refunds",
			"disputes",
			"dunning",
			"payment_intents",
			"setup_intents",
			"customer_balance",
			"payouts",
			"webhooks",
			"billing_events",
			"customer_portal",
		},
		"billingModels": []string{
			"flat_rate",
			"per_seat",
			"metered",
			"tiered",
			"volume",
			"threshold",
			"hybrid",
		},
		"taxCalculation":             true,
		"bankTransferReconciliation": false, // future
	})
}

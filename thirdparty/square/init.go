package square

import (
	"os"
	"strings"

	"github.com/hanzoai/commerce/payment/processor"
)

func init() {
	// Idempotent: if payment/providers/square has already registered a
	// richer Provider (the unified per-tenant shape BD expects), do
	// nothing. Only self-register when this package is imported standalone
	// (api/checkout/square/*) without the unified provider loaded.
	if existing, err := processor.Get(processor.Square); err == nil && existing != nil {
		// If the already-registered processor is this package's
		// SquareProcessor, we leave it alone (re-init is a no-op).
		// Otherwise a unified provider has claimed the slot and we
		// defer to it — BD will Configure() it per-tenant from KMS.
		return
	}

	env := strings.ToLower(strings.TrimSpace(os.Getenv("SQUARE_ENVIRONMENT")))
	sandbox := SandboxEnv()
	creds := FromEnv(sandbox)
	webhookKey := strings.TrimSpace(os.Getenv("SQUARE_WEBHOOK_SIGNATURE_KEY"))
	// Exact dashboard notification URL — Square signs HMAC over URL+body.
	webhookURL := strings.TrimSpace(os.Getenv("SQUARE_WEBHOOK_URL"))

	if sandbox {
		env = "sandbox"
	} else if env == "" {
		env = "production"
	}

	p := NewProcessor(Config{
		AccessToken:   creds.AccessToken,
		LocationID:    creds.LocationID,
		WebhookSecret: webhookKey,
		WebhookURL:    webhookURL,
		Environment:   env,
	})

	processor.Register(p)
}

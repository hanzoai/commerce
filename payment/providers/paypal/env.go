package paypal

import (
	"os"
	"strings"
)

// Credentials holds PayPal REST API v2 credentials of one kind (sandbox or live),
// read from the deployment environment.
type Credentials struct {
	ClientID      string
	ClientSecret  string
	ApplicationID string
	WebhookID     string
}

// FromEnv reads the deployment's PayPal credentials of one kind: the
// PAYPAL_SANDBOX_* names for sandbox, the PAYPAL_* / PAYPAL_LIVE_* names for
// live.
//
// The two sets never mix. A sandbox read consults no live variable, and a live
// read consults no sandbox variable, so a sandbox org can never send live
// credentials to sandbox or charge a live merchant account.
func FromEnv(sandbox bool) Credentials {
	if sandbox {
		return Credentials{
			ClientID:      envValue("PAYPAL_SANDBOX_CLIENT_ID"),
			ClientSecret:  envValue("PAYPAL_SANDBOX_CLIENT_SECRET"),
			ApplicationID: envValue("PAYPAL_SANDBOX_APPLICATION_ID"),
			WebhookID:     envValue("PAYPAL_SANDBOX_WEBHOOK_ID"),
		}
	}
	return Credentials{
		ClientID:      firstEnv("PAYPAL_CLIENT_ID", "PAYPAL_LIVE_CLIENT_ID"),
		ClientSecret:  firstEnv("PAYPAL_CLIENT_SECRET", "PAYPAL_LIVE_CLIENT_SECRET"),
		ApplicationID: firstEnv("PAYPAL_APPLICATION_ID", "PAYPAL_LIVE_APPLICATION_ID"),
		WebhookID:     firstEnv("PAYPAL_WEBHOOK_ID", "PAYPAL_LIVE_WEBHOOK_ID"),
	}
}

// SandboxEnv reports whether the deployment's PAYPAL_ENVIRONMENT names PayPal's
// sandbox ("sandbox" or "test", any case).
func SandboxEnv() bool {
	env := strings.ToLower(envValue("PAYPAL_ENVIRONMENT"))
	return env == "sandbox" || env == "test"
}

func envValue(name string) string { return strings.TrimSpace(os.Getenv(name)) }

func firstEnv(names ...string) string {
	for _, n := range names {
		if v := envValue(n); v != "" {
			return v
		}
	}
	return ""
}

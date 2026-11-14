package paypal

import (
	"strings"

	"github.com/hanzoai/commerce/payment/processor"
)

// Credentials holds PayPal REST API v2 credentials of one kind (sandbox or live),
// read from the deployment environment.
type Credentials struct {
	ClientID      string
	ClientSecret  string
	ApplicationID string
	WebhookID     string
}

// Deployment is the deployment's PayPal credentials of one kind, from the routing policy's
// paypal accounts: the "sandbox" account for sandbox, the "live" account for live. The
// client secret is read from KMS at the account's secret path; the client, application
// and webhook ids are plain settings.
//
// The two sets never mix. A sandbox read consults no live account, and a live read
// consults no sandbox account, so a sandbox org can never send live credentials to
// sandbox or charge a live merchant account.
func Deployment(sandbox bool) Credentials {
	id := "live"
	if sandbox {
		id = "sandbox"
	}
	a := processor.AccountOf(processor.PayPal, id)
	return Credentials{
		ClientID:      strings.TrimSpace(a.Setting("clientId")),
		ClientSecret:  strings.TrimSpace(a.Secret("clientSecret")),
		ApplicationID: strings.TrimSpace(a.Setting("applicationId")),
		WebhookID:     strings.TrimSpace(a.Setting("webhookId")),
	}
}

// SandboxEnv reports whether the deployment's own PayPal account is the sandbox one: the
// account the policy marks as the deployment's (setting deployment=true), else live.
func SandboxEnv() bool {
	return processor.AccountOf(processor.PayPal, "sandbox").Setting("deployment") == "true"
}

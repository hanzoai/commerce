package mpc

import (
	"os"
	"strings"

	"github.com/hanzoai/commerce/payment/processor"
)

func init() {
	kmsEndpoint := strings.TrimSpace(os.Getenv("MPC_KMS_ENDPOINT"))
	if kmsEndpoint == "" {
		kmsEndpoint = "https://kms.hanzo.ai"
	}
	mpcEndpoint := strings.TrimSpace(os.Getenv("MPC_ENDPOINT"))
	apiKey := strings.TrimSpace(os.Getenv("MPC_API_KEY"))

	// The secret the MPC service signs its deliveries with, separate from the
	// API key we authenticate outbound calls with. Unset means inbound webhooks
	// are refused, so turning the rail on means provisioning both.
	webhookSecret := strings.TrimSpace(os.Getenv("MPC_WEBHOOK_SECRET"))

	p := NewProcessor(Config{
		KMSEndpoint:   kmsEndpoint,
		MPCEndpoint:   mpcEndpoint,
		APIKey:        apiKey,
		WebhookSecret: webhookSecret,
	})
	processor.Register(p)
}

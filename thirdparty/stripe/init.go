package stripe

import (
	"os"
	"strings"

	"github.com/hanzoai/commerce/payment/processor"
)

func init() {
	env := strings.ToLower(strings.TrimSpace(os.Getenv("ENV")))
	isTest := env == "test" || env == "development" || env == "sandbox"

	var accessToken string
	if isTest {
		accessToken = strings.TrimSpace(os.Getenv("STRIPE_TEST_SECRET_KEY"))
	}
	if accessToken == "" {
		accessToken = strings.TrimSpace(os.Getenv("STRIPE_LIVE_SECRET_KEY"))
	}

	webhookSecret := strings.TrimSpace(os.Getenv("STRIPE_WEBHOOK_SECRET"))

	p := NewProcessor(accessToken, webhookSecret)
	processor.Register(p)
}

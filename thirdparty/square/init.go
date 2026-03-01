package square

import (
	"os"
	"strings"

	"github.com/hanzoai/commerce/payment/processor"
)

func init() {
	env := strings.ToLower(strings.TrimSpace(os.Getenv("SQUARE_ENVIRONMENT")))
	isSandbox := env == "sandbox" || env == "test"

	token := strings.TrimSpace(os.Getenv("SQUARE_ACCESS_TOKEN"))
	locationID := strings.TrimSpace(os.Getenv("SQUARE_LOCATION_ID"))
	appID := strings.TrimSpace(os.Getenv("SQUARE_APPLICATION_ID"))
	webhookKey := strings.TrimSpace(os.Getenv("SQUARE_WEBHOOK_SIGNATURE_KEY"))

	if isSandbox {
		if t := strings.TrimSpace(os.Getenv("SQUARE_SANDBOX_ACCESS_TOKEN")); t != "" {
			token = t
		}
		if l := strings.TrimSpace(os.Getenv("SQUARE_SANDBOX_LOCATION_ID")); l != "" {
			locationID = l
		}
		if a := strings.TrimSpace(os.Getenv("SQUARE_SANDBOX_APPLICATION_ID")); a != "" {
			appID = a
		}
		env = "sandbox"
	} else if env == "" {
		env = "production"
	}

	_ = appID // ApplicationID stored for future use (OAuth flows, etc.)

	p := NewProcessor(Config{
		AccessToken:   token,
		LocationID:    locationID,
		WebhookSecret: webhookKey,
		Environment:   env,
	})

	processor.Register(p)
}

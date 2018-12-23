package config

import (
	"path/filepath"

	"hanzo.io/types/email"
	"hanzo.io/types/integration"
)

// Default settings
func Defaults() *Config {
	config := new(Config)

	config.ProjectId = "crowdstart-us"
	config.Protocol = "//" // Protocol relative
	config.Hosts = make(map[string]string, 10)
	config.Prefixes = make(map[string]string, 10)
	config.RootDir, _ = filepath.Abs(cwd + "/../..")
	config.SiteTitle = "Hanzo"

	config.Fee = 0.05

	config.Secret = "change-me-in-production"
	config.SessionName = "session"

	config.DemoMode = demoMode

	config.Email = struct {
		From     email.Email
		ReplyTo  email.Email
		Provider integration.Integration
	}{
		email.Email{
			Name:    "Hanzo",
			Address: "platform@hanzo.ai",
		},
		email.Email{
			Name:    "Hanzo",
			Address: "noreply@hanzo.ai",
		},
		integration.Integration{
			Mandrill: integration.Mandrill{
				APIKey: "",
			},
		},
	}

	config.Ethereum.TestPassword = ""
	config.Ethereum.DepositPassword = ""
	// Parity
	// config.Ethereum.MainNetNodes = []string{"http://35.192.92.62:13264"}
	// Geth
	config.Ethereum.MainNetNodes = []string{"http://35.193.184.247:13264"}
	config.Ethereum.TestNetNodes = []string{"https://api.infura.io/v1/jsonrpc/ropsten"}
	// config.Ethereum.TestNetNodes = []string{"http://35.192.74.139:13264"}
	config.Ethereum.WebhookPassword = ""

	config.Bitcoin.TestPassword = ""
	config.Bitcoin.DepositPassword = ""
	config.Bitcoin.MainNetNodes = []string{"http://35.192.49.112:19283"}
	config.Bitcoin.MainNetUsernames = []string{""}
	config.Bitcoin.MainNetPasswords = []string{""}
	config.Bitcoin.TestNetNodes = []string{"http://104.154.51.133:19283"}
	config.Bitcoin.TestNetUsernames = []string{""}
	config.Bitcoin.TestNetPasswords = []string{""}
	config.Bitcoin.WebhookPassword = ""

	config.Paypal.Email = "dev@hanzo.ai"
	config.Paypal.Api = "https://svcs.sandbox.paypal.com"
	config.Paypal.IpnUrl = "https://api.staging.hanzo.io/paypal/ipn/"
	config.Paypal.PaypalIpnUrl = "https://www.sandbox.paypal.com/cgi-bin/webscr"

	config.Stripe.BankAccount = "ba_14trEsCSRlllXCwPzT8vGYiK"
	config.Stripe.DevelopmentClientId = "ca_53yyPzxlPsdAtzMEIuS2mXYDp4FFXLmm"
	config.Stripe.ProductionClientId = "ca_53yyRUNpMtTRUgMlVlLAM3vllY1AVybU"

	config.Stripe.TestSecretKey = ""
	config.Stripe.TestPublishableKey = "pk_test_ucSTeAAtkSXVEg713ir40UhX"
	config.Stripe.LiveSecretKey = ""
	config.Stripe.LivePublishablKey = "pk_live_APr2mdiUblcOO4c2qTeyQ3hq"

	config.Mandrill.FromName = "Hanzo"
	config.Mandrill.FromEmail = "noreply@hanzo.io"

	config.Redis.Url = "pub-redis-19324.us-central1-1-1.gce.garantiadata.com:19324"
	config.Redis.Password = ""

	config.Netlify.BaseUrl = "https://api.netlify.com/api/v1"
	config.Netlify.ClientId = ""
	config.Netlify.Secret = ""

	config.Cloudflare.Email = "dev@hanzo.ai"
	config.Cloudflare.Key = ""
	config.Cloudflare.Zone = "hanzo.io"

	config.SMTPRelay.Endpoint = "https://smtprelay.hanzo.ai"
	config.SMTPRelay.Username = "admin@hanzo.ai"
	config.SMTPRelay.Password = ""

	return config
}

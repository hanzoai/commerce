package config

import (
	"appengine"
	"os"
	"strings"
)

var demoMode = true

type Config struct {
	DemoMode          bool
	Development       bool
	Production        bool
	AutoCompileAssets bool
	RootDir           string
	StaticUrl         string
	Prefixes          map[string]string
	Hosts             map[string]string
	Stripe            struct {
		ClientId    string
		APIKey      string
		APISecret   string
		RedirectURL string
		WebhookURL  string
	}
}

func (c Config) PrefixFor(moduleName string) string {
	return c.Prefixes[moduleName]
}

func (c Config) URLFor(moduleName, domain string) string {
	// Build URL for module.
	url := c.Hosts[moduleName] + c.PrefixFor(moduleName)

	// If module is hosted, return relative to that root domain.
	if domain != "" {
		url = strings.Replace(url, "crowdstart.io", domain, 1)

	}

	return url
}

func Defaults() *Config {
	config := new(Config)
	config.Hosts = make(map[string]string, 10)
	config.Prefixes = make(map[string]string, 10)
	config.RootDir, _ = os.Getwd()
	config.DemoMode = demoMode
	return config
}

func Development() *Config {
	config := Defaults()
	config.AutoCompileAssets = false
	config.Development = true

	config.Prefixes["api"] = "/v1/"
	config.Prefixes["checkout"] = "/checkout/"
	config.Prefixes["platform"] = "/admin/"
	config.Prefixes["preorder"] = "/preorder/"
	config.Prefixes["store"] = "/"

	config.Hosts["api"] = "localhost:8080"
	config.Hosts["checkout"] = "localhost:8080"
	config.Hosts["platform"] = "localhost:8080"
	config.Hosts["preorder"] = "localhost:8080"
	config.Hosts["store"] = "localhost:8080"

	config.StaticUrl = "/static"

	config.Stripe.ClientId = "ca_53yyPzxlPsdAtzMEIuS2mXYDp4FFXLmm"
	config.Stripe.APIKey = "pk_test_ucSTeAAtkSXVEg713ir40UhX"
	config.Stripe.APISecret = ""
	config.Stripe.RedirectURL = "http://localhost:8080/admin/stripe/callback"
	config.Stripe.WebhookURL = "http://localhost:8080/admin/stripe/hook"
	return config
}

func Production() *Config {
	config := Defaults()

	config.Production = true

	config.Prefixes["api"] = "/v1"
	config.Prefixes["checkout"] = "/"
	config.Prefixes["platform"] = "/"
	config.Prefixes["preorder"] = "/"
	config.Prefixes["store"] = "/"

	config.Hosts["api"] = "api.crowdstart.io"
	config.Hosts["checkout"] = "secure.crowdstart.io"
	config.Hosts["platform"] = "platform.crowdstart.io"
	config.Hosts["preorder"] = "preorder.crowdstart.io"
	config.Hosts["store"] = "store.crowdstart.io"

	config.StaticUrl = "//static.crowdstart.io"

	// Only use production credentials if demo mode is off.
	if !config.DemoMode {
		config.Stripe.ClientId = "ca_53yyRUNpMtTRUgMlVlLAM3vllY1AVybU"
		config.Stripe.APIKey = "pk_live_APr2mdiUblcOO4c2qTeyQ3hq"
		config.Stripe.APISecret = ""
		config.Stripe.RedirectURL = "https://secure.crowdstart.io/admin/stripe/callback"
		config.Stripe.WebhookURL = "https://secure.crowdstart.io/admin/stripe/hook"
	}

	return config
}

func Get() *Config {
	if appengine.IsDevAppServer() {
		return Development()
	} else {
		return Production()
	}
}

package config

import (
	"appengine"
	"os"
	"path/filepath"
	"strings"
)

var demoMode = true
var cachedConfig *Config

type Config struct {
	DemoMode          bool
	IsDevelopment     bool
	IsProduction      bool
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

// Return routing prefix for module
func (c Config) PrefixFor(moduleName string) string {
	return c.Prefixes[moduleName]
}

// Return full url to module
func (c Config) ModuleUrl(moduleName, domain string) string {
	// Build URL for module.
	url := c.Hosts[moduleName] + c.PrefixFor(moduleName)

	// If module is hosted, return relative to that root domain.
	if domain != "" {
		url = strings.Replace(url, "crowdstart.io", domain, 1)
	}

	return url
}

// Default settings
func Defaults() *Config {
	cwd, _ := os.Getwd()
	config := new(Config)
	config.Hosts = make(map[string]string, 10)
	config.Prefixes = make(map[string]string, 10)
	config.RootDir, _ = filepath.Abs(cwd + "/../..")
	config.DemoMode = demoMode
	return config
}

// Development settings
func Development() *Config {
	config := Defaults()
	config.IsDevelopment = true

	config.AutoCompileAssets = false

	config.Prefixes["default"] = "/"
	config.Prefixes["api"] = "/api/"
	config.Prefixes["checkout"] = "/checkout/"
	config.Prefixes["platform"] = "/admin/"
	config.Prefixes["preorder"] = "/preorder/"
	config.Prefixes["store"] = "/store/"

	config.Hosts["default"] = "localhost:8080"
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

// Production Settings
func Production() *Config {
	config := Defaults()

	config.IsProduction = true

	config.Prefixes["default"] = "/"
	config.Prefixes["api"] = "/"
	config.Prefixes["checkout"] = "/"
	config.Prefixes["platform"] = "/"
	config.Prefixes["preorder"] = "/"
	config.Prefixes["store"] = "/"

	config.Hosts["default"] = "static.crowdstart.io"
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

// Get current config object
func Get() *Config {
	if cachedConfig != nil {
		return cachedConfig
	}
	if appengine.IsDevAppServer() {
		cachedConfig = Development()
	} else {
		cachedConfig = Production()
	}

	return cachedConfig
}

var config = Get()

var DemoMode = config.DemoMode
var IsDevelopment = config.IsDevelopment
var IsProduction = config.IsProduction
var AutoCompileAssets = config.AutoCompileAssets
var RootDir = config.RootDir
var StaticUrl = config.StaticUrl
var Stripe = config.Stripe

func PrefixFor(moduleName string) string {
	return config.PrefixFor(moduleName)
}

// Return full url to module
func ModuleUrl(moduleName, domain string) string {
	return config.ModuleUrl(moduleName, domain)
}

package config

import (
	"appengine"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"crowdstart.io/util/log"
)

var demoMode = true
var cachedConfig *Config

// CWD is set to config/development due to how we split development/production
// app.yaml files so we need to check two places for config.json based on which
// module is trying to load it.
var cwd, _ = os.Getwd()
var configFileLocations = []string{cwd + "/../config.json", cwd + "/../../config.json"}

type Config struct {
	DemoMode          bool
	IsDevelopment     bool
	IsProduction      bool
	AutoCompileAssets bool
	AutoLoadFixtures  bool
	RootDir           string
	StaticUrl         string
	SiteTitle         string
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
func (c Config) ModuleUrl(moduleName string, args ...interface{}) string {

	// Build protocol-relative URL for module.
	url := "//" + c.Hosts[moduleName] + c.PrefixFor(moduleName)

	for i, arg := range args {
		switch i {
		case 0:
			domain := arg.(string)
			// If module is hosted, return relative to that root domain.
			if domain != "" {
				url = strings.Replace(url, "crowdstart.io", domain, 1)
			}
		}
	}

	// Strip trailing slash
	return strings.TrimRight(url, "/")
}

func (c Config) UrlFor(moduleName string, path string, args ...interface{}) string {
	return c.ModuleUrl(moduleName, args...) + path
}

// Load configuration from JSON file
func (c *Config) Load(fileName string) {
	file, err := os.Open(fileName)
	if err != nil {
		log.Panic("Failed to open configuration file: %v", err)
	}
	decoder := json.NewDecoder(file)
	if err = decoder.Decode(c); err != nil {
		log.Panic("Failed to decode configuration file: %v", err)
	}
}

// Default settings
func Defaults() *Config {
	config := new(Config)
	config.Hosts = make(map[string]string, 10)
	config.Prefixes = make(map[string]string, 10)
	config.RootDir, _ = filepath.Abs(cwd + "/../..")
	config.SiteTitle = "SKULLY"
	config.DemoMode = demoMode
	return config
}

// Development settings
func Development() *Config {
	config := Defaults()

	config.IsDevelopment = true

	config.AutoCompileAssets = false
	config.AutoLoadFixtures = true

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
	config.Stripe.RedirectURL = "http:" + config.ModuleUrl("platform") + "/stripe/callback"
	config.Stripe.WebhookURL = "http:" + config.ModuleUrl("platform") + "/stripe/hook"
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
	config.Hosts["platform"] = "admin.crowdstart.io"
	config.Hosts["preorder"] = "preorder.crowdstart.io"
	config.Hosts["store"] = "store.crowdstart.io"

	config.StaticUrl = "//static.crowdstart.io"

	// Only use production credentials if demo mode is off.
	if !config.DemoMode {
		config.Stripe.ClientId = "ca_53yyRUNpMtTRUgMlVlLAM3vllY1AVybU"
		config.Stripe.APIKey = "pk_live_APr2mdiUblcOO4c2qTeyQ3hq"
		config.Stripe.APISecret = ""
		config.Stripe.RedirectURL = "https:" + config.ModuleUrl("platform") + "/stripe/callback"
		config.Stripe.WebhookURL = "https:" + config.ModuleUrl("platform") + "/stripe/hook"
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

	// Allow local config file to override settings

	for _, configFile := range configFileLocations {
		if _, err := os.Stat(configFile); err == nil {
			cachedConfig.Load(configFile)
		}
	}

	return cachedConfig
}

var config = Get()

// Expose global config.
var DemoMode = config.DemoMode
var IsDevelopment = config.IsDevelopment
var IsProduction = config.IsProduction
var AutoCompileAssets = config.AutoCompileAssets
var AutoLoadFixtures = config.AutoLoadFixtures
var RootDir = config.RootDir
var StaticUrl = config.StaticUrl
var Stripe = config.Stripe
var SiteTitle = config.SiteTitle

func PrefixFor(moduleName string) string {
	return config.PrefixFor(moduleName)
}

// Return full url to module
func ModuleUrl(moduleName string, args ...interface{}) string {
	return config.ModuleUrl(moduleName, args...)
}

func UrlFor(moduleName string, path string, args ...interface{}) string {
	return config.UrlFor(moduleName, path, args...)
}

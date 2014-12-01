package config

import (
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"strings"

	"appengine"

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
	Salesforce        struct {
		ConsumerKey    string
		ConsumerSecret string
		CallbackURL    string
	}
	Stripe struct {
		ClientId    string
		APIKey      string
		APISecret   string
		RedirectURL string
		WebhookURL  string
	}

	Mandrill struct {
		ApiKey string
	}
}

// Return url to static file, module or path rooted in a module
func (c Config) UrlFor(moduleName string, args ...string) (url string) {
	// If we find `moduleName`, we'll use that as root, otherwise assume we
	// were passed a static file as `moduleName`.
	if host, ok := c.Hosts[moduleName]; ok {
		// Use host + prefix to build url root to path in given module
		url = host + c.Prefixes[moduleName]
		args = append([]string{url}, args...)
	} else {
		url = c.StaticUrl
		args = append([]string{url, moduleName}, args...)
	}

	// Join all parts of the path
	url = path.Join(args...)

	// Strip leading slash and replace with protocol relative leading "//".
	url = "//" + strings.TrimLeft(url, "/")

	// Add back ending "/" if trimmed.
	if len(args) > 0 {
		last := args[len(args)-1]
		if string(last[len(last)-1]) == "/" {
			url = url + "/"
		}
	}

	return url
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
	config.Mandrill.ApiKey = "wJ3LGLp5ZOUZlSH8wwqmTg"
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

	config.StaticUrl = "localhost:8080/static"

	// TODO: Create dev versions somehow
	config.Salesforce.ConsumerKey = "3MVG9xOCXq4ID1uElRYWhpUWjXSbiTVg4WO6q9DvWdvBjQ_DFlwSc7jZ9AbY3z9Jv_V29W7xq1nPjTYQhYJqF"
	config.Salesforce.ConsumerSecret = "3811316853831925498"
	config.Salesforce.CallbackURL = "https://admin.crowdstart.io/salesforce/callback"

	config.Stripe.ClientId = "ca_53yyPzxlPsdAtzMEIuS2mXYDp4FFXLmm"
	config.Stripe.APIKey = "pk_test_ucSTeAAtkSXVEg713ir40UhX"
	config.Stripe.APISecret = ""
	config.Stripe.RedirectURL = "http:" + config.UrlFor("platform", "/stripe/callback")
	config.Stripe.WebhookURL = "http:" + config.UrlFor("platform", "/stripe/hook")
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
		config.Salesforce.ConsumerKey = "3MVG9xOCXq4ID1uElRYWhpUWjXSbiTVg4WO6q9DvWdvBjQ_DFlwSc7jZ9AbY3z9Jv_V29W7xq1nPjTYQhYJqF"
		config.Salesforce.ConsumerSecret = "3811316853831925498"
		config.Salesforce.CallbackURL = "https://admin.crowdstart.io/salesforce/callback"

		config.Stripe.ClientId = "ca_53yyRUNpMtTRUgMlVlLAM3vllY1AVybU"
		config.Stripe.APIKey = "pk_live_APr2mdiUblcOO4c2qTeyQ3hq"
		config.Stripe.APISecret = ""
		config.Stripe.RedirectURL = "https:" + config.UrlFor("platform", "/stripe/callback")
		config.Stripe.WebhookURL = "https:" + config.UrlFor("platform", "/stripe/hook")
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
var Prefixes = config.Prefixes
var StaticUrl = config.StaticUrl
var Salesforce = config.Salesforce
var Stripe = config.Stripe
var SiteTitle = config.SiteTitle
var Mandrill = config.Mandrill

func UrlFor(moduleName string, args ...string) string {
	return config.UrlFor(moduleName, args...)
}

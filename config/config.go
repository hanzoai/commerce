package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"

	"appengine"
)

var demoMode = true
var cachedConfig *Config

// CWD is set to config/development due to how we split development/production
// app.yaml files so we need to check two places for config.json based on which
// module is trying to load it.
var cwd, _ = os.Getwd()
var configFileLocations = []string{
	cwd + "/../../../../config.json",
	cwd + "/../../../config.json",
	cwd + "/../../config.json",
	cwd + "/../config.json",
	cwd + "/config.json",
}

type Config struct {
	AutoCompileAssets bool
	AutoLoadFixtures  bool
	CookieDomain      string
	DatastoreWarn     bool
	DemoMode          bool
	IsDevelopment     bool
	IsProduction      bool
	IsStaging         bool
	Protocol          string
	RootDir           string
	SentryDSN         string
	SiteTitle         string
	StaticUrl         string

	Secret      string
	SessionName string

	Prefixes map[string]string
	Hosts    map[string]string

	Salesforce struct {
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
		APIKey    string
		FromEmail string
		FromName  string
	}

	Google struct {
		APIKey string
		Bucket struct {
			ImageUploads string
		}
	}
}

// Return url to static file, module or path rooted in a module
func (c Config) UrlFor(moduleName string, args ...string) (url string) {
	// Ignore the port number and the host during testing.
	if c.IsDevelopment && !strings.HasPrefix(moduleName, "/") {
		if len(args) > 0 {
			return "/" + moduleName + args[0]
		} else {
			return "/" + moduleName + "/"
		}
	}

	// Trim whitespace
	moduleName = strings.TrimSpace(moduleName)

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
	url = c.Protocol + strings.TrimLeft(url, "/")

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
		panic(fmt.Sprintf("Failed to open configuration file: %v", err))
	}
	decoder := json.NewDecoder(file)
	if err = decoder.Decode(c); err != nil {
		panic(fmt.Sprintf("Failed to decode configuration file: %v", err))
	}
}

// Get current config object
func Get() *Config {
	if cachedConfig != nil {
		return cachedConfig
	}

	if appengine.IsDevAppServer() {
		cachedConfig = Development()
	} else {
		// TODO: This is a total hack, probably can't rely on this.
		// Use PWD to determine appid, if s~crowdstart-io-staging is in PWD,
		// then we're in staging enviroment.
		pwd := os.Getenv("PWD")
		if strings.Contains(pwd, "s~crowdstart-staging") {
			cachedConfig = Staging()
		} else if strings.Contains(pwd, "s~crowdstart-skully") {
			cachedConfig = Skully()
		} else {
			cachedConfig = Production()
		}
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
var AutoCompileAssets = config.AutoCompileAssets
var AutoLoadFixtures = config.AutoLoadFixtures
var CookieDomain = config.CookieDomain
var DatastoreWarn = config.DatastoreWarn
var DemoMode = config.DemoMode
var Google = config.Google
var IsDevelopment = config.IsDevelopment
var IsProduction = config.IsProduction
var IsStaging = config.IsStaging
var Mandrill = config.Mandrill
var Prefixes = config.Prefixes
var RootDir = config.RootDir
var Salesforce = config.Salesforce
var Secret = config.Secret
var SentryDSN = config.SentryDSN
var SessionName = config.SessionName
var SiteTitle = config.SiteTitle
var StaticUrl = config.StaticUrl
var Stripe = config.Stripe

func UrlFor(moduleName string, args ...string) string {
	return config.UrlFor(moduleName, args...)
}

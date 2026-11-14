package processor

import (
	"strings"
	"sync"
)

// This file is the ONE table of what a deployment's environment configured, in the names
// it used. It is read in two places and nowhere else: the first run seeds the routing
// policy's accounts from it (api/billing), and a test states a deployment as an EnvSeed.
// Nothing at runtime reads the environment for any processor.

// SeedField is one value an account takes from the environment on the first run: its name
// in the account, the environment names it may have had (the first one set wins), and
// whether it is a secret (then the account holds the KMS path it was provisioned at).
type SeedField struct {
	Name   string
	Envs   []string
	Secret bool
}

// SeedSpec is one account of a processor and the environment it was configured by.
type SeedSpec struct {
	Processor, ID string
	Fields        []SeedField
	// deployment names the environment variable and the values that make this account the
	// deployment's own (Square and PayPal keep a live and a sandbox set side by side).
	Deployment string
	DeployWhen []string
}

var Seeds = []SeedSpec{
	{Processor: "stripe", ID: "live", Fields: []SeedField{
		{"secretKey", []string{"STRIPE_SECRET_KEY"}, true}, {"webhook", []string{"STRIPE_WEBHOOK_SECRET"}, true}, {"publishableKey", []string{"STRIPE_PUBLISHABLE_KEY"}, false}}},
	{Processor: "stripe", ID: "test", Fields: []SeedField{
		{"secretKey", []string{"STRIPE_TEST_SECRET_KEY"}, true}, {"publishableKey", []string{"STRIPE_TEST_PUBLISHABLE_KEY"}, false}}},
	{Processor: "square", ID: "live", Deployment: "SQUARE_ENVIRONMENT", DeployWhen: []string{"", "production", "live"}, Fields: []SeedField{
		{"accessToken", []string{"SQUARE_ACCESS_TOKEN"}, true}, {"locationId", []string{"SQUARE_LOCATION_ID"}, false}, {"applicationId", []string{"SQUARE_APPLICATION_ID"}, false},
		{"webhookSignature", []string{"SQUARE_WEBHOOK_SIGNATURE_KEY"}, true}, {"webhookUrl", []string{"SQUARE_WEBHOOK_URL"}, false}}},
	{Processor: "square", ID: "sandbox", Deployment: "SQUARE_ENVIRONMENT", DeployWhen: []string{"sandbox", "test"}, Fields: []SeedField{
		{"accessToken", []string{"SQUARE_SANDBOX_ACCESS_TOKEN"}, true}, {"locationId", []string{"SQUARE_SANDBOX_LOCATION_ID"}, false}, {"applicationId", []string{"SQUARE_SANDBOX_APPLICATION_ID"}, false}}},
	{Processor: "paypal", ID: "live", Deployment: "PAYPAL_ENVIRONMENT", DeployWhen: []string{"", "production", "live"}, Fields: []SeedField{
		{"clientId", []string{"PAYPAL_CLIENT_ID", "PAYPAL_LIVE_CLIENT_ID"}, false}, {"clientSecret", []string{"PAYPAL_CLIENT_SECRET", "PAYPAL_LIVE_CLIENT_SECRET"}, true},
		{"applicationId", []string{"PAYPAL_APPLICATION_ID", "PAYPAL_LIVE_APPLICATION_ID"}, false}, {"webhookId", []string{"PAYPAL_WEBHOOK_ID", "PAYPAL_LIVE_WEBHOOK_ID"}, false}}},
	{Processor: "paypal", ID: "sandbox", Deployment: "PAYPAL_ENVIRONMENT", DeployWhen: []string{"sandbox", "test"}, Fields: []SeedField{
		{"clientId", []string{"PAYPAL_SANDBOX_CLIENT_ID"}, false}, {"clientSecret", []string{"PAYPAL_SANDBOX_CLIENT_SECRET"}, true},
		{"applicationId", []string{"PAYPAL_SANDBOX_APPLICATION_ID"}, false}, {"webhookId", []string{"PAYPAL_SANDBOX_WEBHOOK_ID"}, false}}},
	{Processor: "moonpay", ID: "default", Fields: []SeedField{
		{"apiKey", []string{"MOONPAY_API_KEY"}, true}, {"secretKey", []string{"MOONPAY_SECRET_KEY"}, true}, {"webhookKey", []string{"MOONPAY_WEBHOOK_KEY"}, true}}},
	{Processor: "opennode", ID: "default", Fields: []SeedField{
		{"apiKey", []string{"OPENNODE_API_KEY"}, true}, {"environment", []string{"OPENNODE_ENVIRONMENT"}, false}}},
	{Processor: "bitpay", ID: "default", Fields: []SeedField{
		{"apiToken", []string{"BITPAY_API_TOKEN"}, true}, {"environment", []string{"BITPAY_ENVIRONMENT"}, false}}},
	{Processor: "coinbase_commerce", ID: "default", Fields: []SeedField{
		{"apiKey", []string{"COINBASE_COMMERCE_API_KEY"}, true}, {"webhook", []string{"COINBASE_COMMERCE_WEBHOOK_SECRET"}, true}}},
	{Processor: "solanapay", ID: "default", Fields: []SeedField{
		{"rpcUrl", []string{"SOLANA_RPC_URL"}, false}, {"recipient", []string{"SOLANA_RECIPIENT_ADDRESS"}, false}}},
}

// Deploys says the environment names this account as the deployment's own.
func (s SeedSpec) Deploys(env map[string]string) bool {
	if s.Deployment == "" {
		return false
	}
	named := strings.ToLower(strings.TrimSpace(env[s.Deployment]))
	for _, w := range s.DeployWhen {
		if named == w {
			return true
		}
	}
	return false
}

// EnvSeed is a deployment's environment as a map, for a test to state the accounts it
// runs under. Seeds accumulate, as the variables of a process do: applying a second one
// adds to the first, and the cleanup it returns takes back only its own.
type EnvSeed map[string]string

var (
	seedMu  sync.Mutex
	seedEnv = EnvSeed{}
)

// Apply gives every processor the accounts the accumulated seed describes, with the
// secrets' values taken straight from it (there is no KMS in a test), and returns what
// puts the previous accounts back.
func (e EnvSeed) Apply() (restore func()) {
	seedMu.Lock()
	before := EnvSeed{}
	for k, v := range seedEnv {
		before[k] = v
	}
	for k, v := range e {
		seedEnv[k] = v
	}
	now := before.merged(e)
	seedMu.Unlock()
	restores := now.install()
	return func() {
		seedMu.Lock()
		seedEnv = before
		seedMu.Unlock()
		for i := len(restores) - 1; i >= 0; i-- {
			restores[i]()
		}
		before.install()
	}
}

func (e EnvSeed) merged(more EnvSeed) EnvSeed {
	out := EnvSeed{}
	for k, v := range e {
		out[k] = v
	}
	for k, v := range more {
		out[k] = v
	}
	return out
}

// install sets every processor's accounts from the seed.
func (e EnvSeed) install() (restores []func()) {
	by := map[ProcessorType][]Account{}
	for _, sp := range Seeds {
		a := Account{ID: sp.ID, Settings: map[string]string{}, Secrets: map[string]string{}}
		any := false
		for _, f := range sp.Fields {
			for _, name := range f.Envs {
				v := strings.TrimSpace(e[name])
				if v == "" {
					continue
				}
				if f.Secret {
					a.Secrets[f.Name] = v
				} else {
					a.Settings[f.Name] = v
				}
				any = true
				break
			}
		}
		if !any {
			continue
		}
		if sp.Deploys(e) {
			a.Settings["deployment"] = "true"
		}
		by[ProcessorType(sp.Processor)] = append(by[ProcessorType(sp.Processor)], a)
	}
	for _, t := range []ProcessorType{Stripe, Square, PayPal, MoonPay, OpenNode, BitPay, CoinbaseCommerce, SolanaPay} {
		restores = append(restores, SetAccounts(t, by[t]))
	}
	return restores
}

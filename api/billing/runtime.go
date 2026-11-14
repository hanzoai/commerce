package billing

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hanzoai/commerce/billing/depositledger"
	"github.com/hanzoai/commerce/billing/depositwatch"
	"github.com/hanzoai/commerce/billing/engine"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/routingpolicy"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/payment/providers/kraken"
	"github.com/hanzoai/commerce/payment/routing"
	"github.com/hanzoai/commerce/secrets"
	"github.com/hanzoai/commerce/thirdparty/mpc"
)

// BOOTSTRAP EXCEPTION. The environment is read before the policy store exists, and only for
// what is needed to open it and reach KMS: the store's location (COMMERCE_DATA_DIR,
// COMMERCE_BASE_URL, COMMERCE_BASE_AUX, store.FromEnv) and the KMS client the host builds
// (cmd/commerce/kms.go). Nothing about a processor, a deposit rail or a credential is read
// from the environment after the first run seeded the policy (seedRuntime).
//
// The platform routing policy is the one place the processors commerce charges with,
// and the deposit accounts it watches, are configured. It is applied to what runs the
// moment it is written on this replica, and picked up by the others on a short refresh:
// no restart, no environment.

// runtimeRefresh is how often a replica rereads the platform policy to learn of a change
// another replica made.
const runtimeRefresh = 15 * time.Second

var (
	// applied is the policy the processors are charging under now.
	applied atomic.Pointer[routing.Policy]
	// appliedVersion is the version of the platform policy last applied, so an unchanged
	// policy is not reapplied on every refresh.
	appliedVersion atomic.Int64
	runtimeMu      sync.Mutex
)

// chargeFromPolicy is the processor charge policy for a routing policy: the processors
// it lists as on. The crypto rail's own switch is not a processor.
func chargeFromPolicy(p routing.Policy) map[processor.ProcessorType]bool {
	allowed := map[processor.ProcessorType]bool{}
	for _, r := range p.Processors {
		if r.On && processor.KnownType(r.Processor) {
			allowed[processor.ProcessorType(r.Processor)] = true
		}
	}
	return allowed
}

// InstallRuntime makes the platform policy what the processors charge under and the
// deposit watcher watches, and is called once at boot after the store is up. A store that
// has never held the policy is seeded once from the environment it was configured with
// (seedRuntime), and the environment is not read again.
func InstallRuntime(ctx context.Context) error {
	if err := seedRuntime(ctx); err != nil {
		return err
	}
	pol, version, err := ReadRoutingVersioned(ctx, "")
	if err != nil {
		return err
	}
	processor.SetChargePolicy(func() map[processor.ProcessorType]bool {
		if p := applied.Load(); p != nil {
			return chargeFromPolicy(*p)
		}
		return map[processor.ProcessorType]bool{}
	})
	return applyRuntime(ctx, pol, version)
}

// applyRuntime puts pol under the processors and the watcher. A deposit table that does
// not hold is refused and the previous one keeps running; the charge policy still moves,
// so a switch-off is never held back by an unrelated bad account.
func applyRuntime(ctx context.Context, pol routing.Policy, version int) error {
	runtimeMu.Lock()
	defer runtimeMu.Unlock()
	applied.Store(&pol)
	appliedVersion.Store(int64(version))
	var first error
	if svc := depositledger.Default(); svc != nil {
		if err := svc.Apply(ctx, pol.Accounts); err != nil {
			log.Error("routing: the deposit accounts were not applied, the previous ones keep running: %v", err)
			first = err
		}
	}
	applyAccounts(ctx, pol)
	configureMPC(ctx, pol)
	configureKraken(ctx, pol)
	return first
}

// accountedProcessors are the processors given accounts by the last apply, so a processor
// whose accounts were all removed is cleared.
var accountedProcessors []processor.ProcessorType

// applyAccounts gives each processor its accounts from the policy: every account that is on
// and is not a deposit account (no network), with its settings and its secrets read from
// KMS now. The processors read their configuration from these and from nothing else.
func applyAccounts(ctx context.Context, pol routing.Policy) {
	by := map[processor.ProcessorType][]processor.Account{}
	for _, a := range pol.Accounts {
		if !a.On || a.Network != "" || a.Processor == "" {
			continue
		}
		acct := processor.Account{ID: a.ID, Settings: map[string]string{}, Secrets: map[string]string{}}
		for _, n := range a.Settings {
			acct.Settings[n.Name] = n.Value
		}
		for _, n := range a.Secrets {
			acct.Secrets[n.Name] = secrets.String(ctx, n.Value)
		}
		by[processor.ProcessorType(a.Processor)] = append(by[processor.ProcessorType(a.Processor)], acct)
	}
	for _, t := range accountedProcessors {
		if _, still := by[t]; !still {
			processor.SetAccounts(t, nil)
		}
	}
	accountedProcessors = accountedProcessors[:0]
	for t, accts := range by {
		processor.SetAccounts(t, accts)
		accountedProcessors = append(accountedProcessors, t)
	}
}

// configureMPC points the mpc rail at the policy's mpc service account (the one with no
// network): its endpoint, and the API key at its secret reference. No such account on
// turns the rail off.
func configureMPC(ctx context.Context, pol routing.Policy) {
	p, err := processor.Global().Registered(processor.MPC)
	if err != nil {
		return
	}
	c, ok := p.(interface{ Configure(mpc.Settings) })
	if !ok {
		return
	}
	for _, a := range pol.Accounts {
		if a.Processor == string(processor.MPC) && a.On && a.Network == "" && a.Endpoint != "" {
			st := mpc.Settings{Endpoint: a.Endpoint, KMSEndpoint: routing.Get(a.Endpoints, "kms"), ZAPAddress: routing.Get(a.Endpoints, "zap")}
			if a.SecretRef != "" {
				st.APIKey = secrets.String(ctx, a.SecretRef)
			}
			if ref := routing.Get(a.Secrets, "webhook"); ref != "" {
				st.WebhookSecret = secrets.String(ctx, ref)
			}
			c.Configure(st)
			return
		}
	}
	c.Configure(mpc.Settings{})
}

// configureKraken gives the exchange processor the policy's kraken account: the API
// credentials read from the KMS folder at its secretRef, and its endpoint. No such
// account on leaves it unconfigured.
func configureKraken(ctx context.Context, pol routing.Policy) {
	p, err := processor.Global().Registered(processor.Kraken)
	if err != nil {
		return
	}
	c, ok := p.(interface{ Configure(kraken.Config) })
	if !ok {
		return
	}
	for _, a := range pol.Accounts {
		if a.Processor == string(processor.Kraken) && a.On && a.SecretRef != "" {
			folder := strings.TrimRight(a.SecretRef, "/")
			c.Configure(kraken.Config{APIKey: secrets.String(ctx, folder+"/KRAKEN_API_KEY"), APISecret: secrets.String(ctx, folder+"/KRAKEN_API_SECRET"), APIURL: a.Endpoint, OTP: secrets.String(ctx, folder+"/KRAKEN_OTP")})
			return
		}
	}
	c.Configure(kraken.Config{})
}

// StartRuntimeRefresh rereads the platform policy every runtimeRefresh and applies it
// when its version moved, until stop is called.
func StartRuntimeRefresh() (stop func()) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		t := time.NewTicker(runtimeRefresh)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				pol, v, err := ReadRoutingVersioned(ctx, "")
				if err != nil {
					log.Warn("routing: the platform policy could not be reread: %v", err)
					continue
				}
				if int64(v) != appliedVersion.Load() {
					_ = applyRuntime(ctx, pol, v)
				}
			}
		}
	}()
	return cancel
}

// seedRuntime writes the platform policy's first version from the environment the
// deployment was configured with, once. A policy that already holds the seed mark is
// left alone; after this runs nothing reads the environment for any of it.
//
// What it carries over: the processors COMMERCE_CHARGE_PROCESSORS named (Stripe alone
// where it was unset and a Stripe key was set, every processor where neither was), the
// crypto deposit accounts CRYPTO_DEPOSIT_* described, the MPC endpoint, and an exchange
// account where Kraken keys were set. Secrets are never copied: an account holds the KMS
// path they live at.
func seedRuntime(ctx context.Context) error {
	row, found, err := routingpolicy.Get(ctx, routingpolicy.Platform)
	if err != nil {
		return err
	}
	if found && seeded(row.Rules) {
		return nil
	}
	pol := routing.Default()
	var version int
	by := "system:env-seed"
	if found {
		pol, _ = policyOf(row)
		version = row.Version
		by = row.By
	}
	seedFromEnvironment(ctx, &pol, os.Environ())
	pol = pol.Normal()
	rows := policyRow(pol, by, version)
	rows.Rules = markSeeded(rows.Rules)
	all, err := routingpolicy.Versions(ctx, routingpolicy.Platform)
	if err != nil {
		return err
	}
	next := 1
	if len(all) > 0 {
		next = all[0].N + 1
	}
	pj, _ := json.Marshal(pol)
	if err := routingpolicy.PutVersion(ctx, routingpolicy.Version{Scope: routingpolicy.Platform, N: next, Policy: string(pj), By: "system:env-seed", Note: "seeded once from the deployment's environment"}); err != nil {
		return err
	}
	rows.Version = next
	if err := routingpolicy.Put(ctx, routingpolicy.Platform, rows); err != nil {
		return err
	}
	_, _ = engine.EmitBillingEvent(webhookGuardDB(ctx), "routing.seeded", "routing", routingpolicy.Platform, "",
		map[string]interface{}{"policy": pol, "version": next}, nil)
	return nil
}

func seeded(rules string) bool {
	var x struct {
		Seeded bool `json:"seeded"`
	}
	return json.Unmarshal([]byte(rules), &x) == nil && x.Seeded
}

func unmarkSeeded(rules string) string {
	var m map[string]json.RawMessage
	if json.Unmarshal([]byte(rules), &m) != nil {
		return rules
	}
	delete(m, "seeded")
	b, _ := json.Marshal(m)
	return string(b)
}

func markSeeded(rules string) string {
	var m map[string]json.RawMessage
	if json.Unmarshal([]byte(rules), &m) != nil || m == nil {
		m = map[string]json.RawMessage{}
	}
	m["seeded"] = json.RawMessage("true")
	b, _ := json.Marshal(m)
	return string(b)
}

// seedFromEnvironment reads the deployment's environment into pol. It is the ONLY place
// commerce reads any of these variables, and it runs once.
func seedFromEnvironment(ctx context.Context, pol *routing.Policy, environ []string) {
	env := map[string]string{}
	for _, kv := range environ {
		if k, v, ok := strings.Cut(kv, "="); ok && strings.TrimSpace(v) != "" {
			env[k] = strings.TrimSpace(v)
		}
	}
	// Processors.
	var allowed map[processor.ProcessorType]bool
	switch raw, set := env["COMMERCE_CHARGE_PROCESSORS"]; {
	case set:
		allowed = processor.ChargeNames(raw)
	case env["STRIPE_SECRET_KEY"] != "" || env["STRIPE_TEST_SECRET_KEY"] != "":
		allowed = processor.ChargeNames("stripe")
	default:
		allowed = processor.ChargeNames("*")
	}
	for i, r := range pol.Processors {
		if processor.KnownType(r.Processor) {
			pol.Processors[i].On = allowed[processor.ProcessorType(r.Processor)]
		}
	}
	// The crypto deposit accounts.
	rpc := map[string]string{}
	for k, v := range env {
		if net, ok := strings.CutPrefix(k, "CRYPTO_DEPOSIT_RPC_"); ok {
			rpc[strings.ToLower(net)] = v
		}
	}
	mpcOn := false
	for k, v := range env {
		rest, ok := strings.CutPrefix(k, "CRYPTO_DEPOSIT_TOKEN_")
		if !ok {
			continue
		}
		us := strings.LastIndexByte(rest, '_')
		if us <= 0 {
			continue
		}
		net, asset := strings.ToLower(rest[:us]), strings.ToLower(rest[us+1:])
		a := routing.Account{Processor: string(processor.MPC), ID: net + "-" + asset, Network: net, Asset: asset, Token: v, Endpoint: rpc[net], On: true, Cards: false}
		a.Address = env["CRYPTO_DEPOSIT_ADDRESS_"+strings.ToUpper(net)]
		if n, err := parseInt(env["CRYPTO_DEPOSIT_FEE_"+strings.ToUpper(net)]); err == nil {
			a.FeeCents = n
		}
		if n, err := parseInt(env["CRYPTO_DEPOSIT_SLIPPAGE_"+strings.ToUpper(net)]); err == nil {
			a.SlippageBps = int(n)
		}
		pol.Accounts = append(pol.Accounts, a)
		mpcOn = true
	}
	if ep := env["MPC_ENDPOINT"]; ep != "" {
		a := routing.Account{Processor: string(processor.MPC), ID: "service", Endpoint: ep, On: true}
		if env["MPC_API_KEY"] != "" {
			a.SecretRef = kmsPath(ctx, "MPC_API_KEY")
		}
		if v := env["MPC_KMS_ENDPOINT"]; v != "" {
			a.Endpoints = append(a.Endpoints, routing.Named{Name: "kms", Value: v})
		}
		if v := env["MPC_ZAP_ADDR"]; v != "" {
			a.Endpoints = append(a.Endpoints, routing.Named{Name: "zap", Value: v})
		}
		if env["MPC_WEBHOOK_SECRET"] != "" {
			a.Secrets = append(a.Secrets, routing.Named{Name: "webhook", Value: kmsPath(ctx, "MPC_WEBHOOK_SECRET")})
		}
		pol.Accounts = append(pol.Accounts, a)
		mpcOn = true
	}
	if mpcOn {
		for i := range pol.Processors {
			if pol.Processors[i].Processor == string(processor.MPC) {
				pol.Processors[i].On = allowed[processor.MPC]
			}
		}
	}
	if env["KRAKEN_API_KEY"] != "" && env["KRAKEN_API_SECRET"] != "" {
		pol.Accounts = append(pol.Accounts, routing.Account{Processor: string(processor.Kraken), ID: "exchange", Endpoint: env["KRAKEN_API_URL"], On: true, SecretRef: kmsFolder(ctx, "KRAKEN_API_KEY")})
		for i := range pol.Processors {
			if pol.Processors[i].Processor == string(processor.Kraken) {
				pol.Processors[i].On = allowed[processor.Kraken]
			}
		}
	}
	seedProcessorAccounts(ctx, pol, env)
	// The default crypto processor an operator named leads the rails for any asset.
	if explicit := strings.ToLower(env["COMMERCE_DEFAULT_CRYPTO_PROCESSOR"]); explicit != "" {
		pol.CryptoRails = append([]routing.CryptoRoute{{Asset: "*", Processor: explicit}}, routing.DefaultCryptoRails()...)
	}
}

func parseInt(s string) (int64, error) {
	var n int64
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, os.ErrInvalid
		}
		n = n*10 + int64(c-'0')
	}
	if s == "" {
		return 0, os.ErrInvalid
	}
	return n, nil
}

// validateAccounts is what an account must be to be stored: a processor the policy
// names; a read endpoint that is https; a deposit table the watcher would accept (its
// addresses written the way their chain writes them, confirmations no lower than the
// network's own minimum); and a secret reference that resolves in KMS now. It says why
// not, or "".
func validateAccounts(ctx context.Context, p routing.Policy) string {
	for _, a := range p.Accounts {
		if a.Endpoint != "" {
			u, err := url.Parse(a.Endpoint)
			if err != nil || u.Scheme != "https" || u.Host == "" {
				return "accounts: " + a.Processor + " " + a.ID + " endpoint must be an https URL"
			}
		}
		for _, e := range a.Endpoints {
			if e.Name == "zap" {
				continue // a host:port or a socket path, not a URL
			}
			if u, err := url.Parse(e.Value); err != nil || u.Scheme != "https" || u.Host == "" {
				return "accounts: " + a.Processor + " " + a.ID + " endpoint " + e.Name + " must be an https URL"
			}
		}
		if a.SecretRef != "" {
			if why := resolveRef(ctx, a); why != "" {
				return "accounts: " + a.Processor + " " + a.ID + " " + why
			}
		}
		for _, sc := range a.Secrets {
			if !strings.HasPrefix(sc.Value, "/") || secrets.String(ctx, sc.Value) == "" {
				return "accounts: " + a.Processor + " " + a.ID + " secret " + sc.Name + " " + sc.Value + " does not resolve in KMS"
			}
		}
	}
	if _, err := depositwatch.AssetsFromAccounts(p.Accounts); err != nil {
		return "accounts: " + err.Error()
	}
	return ""
}

// resolveRef checks a secret reference reads a value now. A kraken account's reference
// is the KMS folder holding KRAKEN_API_KEY and KRAKEN_API_SECRET; every other account's
// is the secret itself. The value is never kept or returned.
func resolveRef(ctx context.Context, a routing.Account) string {
	if !strings.HasPrefix(a.SecretRef, "/") {
		return "secretRef must be a KMS path beginning with /"
	}
	refs := []string{a.SecretRef}
	if a.Processor == string(processor.Kraken) {
		refs = []string{strings.TrimRight(a.SecretRef, "/") + "/KRAKEN_API_KEY", strings.TrimRight(a.SecretRef, "/") + "/KRAKEN_API_SECRET"}
	}
	for _, r := range refs {
		if secrets.String(ctx, r) == "" {
			return "secretRef " + r + " does not resolve in KMS"
		}
	}
	return ""
}

// kmsFolders are the KMS paths a deployment's processor secrets live at, from the
// universe manifests that project them: production's are synced from /commerce-secrets
// (cloud-commerce-kms-sync, cloud-stripe-kms-sync) and testnet's from /cloud
// (cloud-env-kms-sync, KMS env testnet). Which network this is is the host's KMS reader's
// concern; the path a secret is at is the one that resolves for it.
var kmsFolders = []string{"/commerce-secrets", "/cloud"}

// kmsFolder is the folder holding name for this deployment: the first of kmsFolders where
// it resolves, else the first (the secret is then not found, and the first run says so).
func kmsFolder(ctx context.Context, name string) string {
	for _, f := range kmsFolders {
		if secrets.String(ctx, f+"/"+name) != "" {
			return f
		}
	}
	log.Warn("routing seed: %s resolves in none of %v; the account will name %s/%s", name, kmsFolders, kmsFolders[0], name)
	return kmsFolders[0]
}

// kmsPath is the path name is at for this deployment.
func kmsPath(ctx context.Context, name string) string { return kmsFolder(ctx, name) + "/" + name }

// seedProcessorAccounts adds an account for every processor the environment configured
// (processor.Seeds): the plain values as settings, and for each secret the KMS path it
// lives at (/commerce-secrets/<NAME>), never the value. An account is made only if one of
// its values was set; Square and PayPal's live and sandbox sets both survive, and the one
// the environment named is marked as the deployment's.
func seedProcessorAccounts(ctx context.Context, pol *routing.Policy, env map[string]string) {
	for _, sp := range processor.Seeds {
		a := routing.Account{Processor: sp.Processor, ID: sp.ID, On: true}
		for _, f := range sp.Fields {
			for _, name := range f.Envs {
				v := env[name]
				if v == "" {
					continue
				}
				if f.Secret {
					a.Secrets = append(a.Secrets, routing.Named{Name: f.Name, Value: kmsPath(ctx, name)})
				} else {
					a.Settings = append(a.Settings, routing.Named{Name: f.Name, Value: v})
				}
				break
			}
		}
		if len(a.Secrets)+len(a.Settings) == 0 {
			continue
		}
		if sp.Deploys(env) {
			a.Settings = append(a.Settings, routing.Named{Name: "deployment", Value: "true"})
		}
		pol.Accounts = append(pol.Accounts, a)
	}
	// Kraken's explicit deposit-method overrides ride on its exchange account.
	for k, v := range env {
		rest, ok := strings.CutPrefix(k, "KRAKEN_DEPOSIT_METHOD_")
		if !ok {
			continue
		}
		parts := strings.Split(strings.ToLower(rest), "_")
		name := "depositMethod." + strings.Join(parts, ".")
		found := false
		for i := range pol.Accounts {
			if pol.Accounts[i].Processor == string(processor.Kraken) && pol.Accounts[i].ID == "exchange" {
				pol.Accounts[i].Settings = append(pol.Accounts[i].Settings, routing.Named{Name: name, Value: v})
				found = true
			}
		}
		if !found {
			pol.Accounts = append(pol.Accounts, routing.Account{Processor: string(processor.Kraken), ID: "exchange", On: true, Settings: []routing.Named{{Name: name, Value: v}}})
		}
	}
}

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

func init() {
	processor.SetHardDisabledPolicy(envDisabledProcessors)
}

func envDisabledProcessors() map[processor.ProcessorType]bool {
	raw := strings.TrimSpace(os.Getenv("COMMERCE_DISABLED_PROCESSORS"))
	if raw == "" {
		return nil
	}
	return processor.ChargeNames(raw)
}

// chargeFromPolicy is the processor charge policy for a routing policy: the processors
// it lists as on. The crypto rail's own switch is not a processor.
func chargeFromPolicy(p routing.Policy) map[processor.ProcessorType]bool {
	allowed := map[processor.ProcessorType]bool{}
	for _, r := range p.Processors {
		t := processor.ProcessorType(r.Processor)
		if r.On && processor.KnownType(r.Processor) && !processor.DisabledByEnv(t) {
			allowed[t] = true
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
	err = applyRuntime(ctx, pol, version)
	reportUnchargeableOnChange(ctx, pol)
	return err
}

// requiredFields are what an account must hold, resolved, for its processor to charge: the
// secrets are read from KMS and the settings must be non-empty.
var requiredFields = map[string]struct{ secrets, settings []string }{
	"stripe":            {secrets: []string{"secretKey"}},
	"square":            {secrets: []string{"accessToken"}, settings: []string{"locationId"}},
	"paypal":            {secrets: []string{"clientSecret"}, settings: []string{"clientId"}},
	"moonpay":           {secrets: []string{"apiKey", "secretKey"}},
	"opennode":          {secrets: []string{"apiKey"}},
	"bitpay":            {secrets: []string{"apiToken"}},
	"coinbase_commerce": {secrets: []string{"apiKey"}},
	"solanapay":         {settings: []string{"recipient"}},
}

// accountReasons says why an account cannot charge, one reason per secret that does not
// resolve (named by its path and why, never its value) and per setting that is empty;
// empty when it can.
func accountReasons(ctx context.Context, a routing.Account) []string {
	req := requiredFields[a.Processor]
	var why []string
	for _, name := range req.secrets {
		ref := routing.Get(a.Secrets, name)
		if ref == "" {
			why = append(why, "secret "+name+" has no KMS path")
			continue
		}
		if d := secrets.Diagnose(ctx, ref); d != "" {
			why = append(why, "secret "+name+" at "+ref+": "+d)
		}
	}
	for _, name := range req.settings {
		if strings.TrimSpace(routing.Get(a.Settings, name)) == "" {
			why = append(why, "setting "+name+" is empty")
		}
	}
	return why
}

// chargeProblems says what keeps the policy from charging as it runs, by what RESOLVES and
// not by what is listed: each processor that is on and charges through an account
// (Stripe, Square, PayPal, ...) and has no account whose required secrets read from KMS and
// whose required settings are set, with each reason; and, when that is true of every one of
// them, that nothing can charge a card.
func chargeProblems(ctx context.Context, pol routing.Policy) []string {
	var problems []string
	on, ready := 0, 0
	for _, r := range pol.Processors {
		if !r.On || processor.DisabledByEnv(processor.ProcessorType(r.Processor)) {
			continue
		}
		if _, needs := requiredFields[r.Processor]; !needs {
			continue
		}
		on++
		var reasons []string
		usable := false
		any := false
		for _, a := range pol.Accounts {
			if a.Processor != r.Processor || !a.On || a.Network != "" {
				continue
			}
			any = true
			why := accountReasons(ctx, a)
			if takesCardsAccount(a.Processor) && !a.Cards {
				why = append(why, "takes no cards")
			}
			if len(why) == 0 {
				usable = true
				break
			}
			for _, w := range why {
				reasons = append(reasons, "account "+a.ID+" "+w)
			}
		}
		switch {
		case usable:
			ready++
		case !any:
			problems = append(problems, r.Processor+" is on and holds no account to charge with")
		default:
			problems = append(problems, r.Processor+" is on and no account of it is able to charge (unchargeable): "+strings.Join(reasons, "; "))
		}
	}
	if on > 0 && ready == 0 {
		problems = append(problems, "no processor that is on can charge: nothing can charge a card")
	}
	return problems
}

// settlementFields are the secrets a processor needs to CONFIRM a payment it took (its webhook
// signature): without one the money can be taken but credits cannot be confirmed.
var settlementFields = map[string][]string{
	"stripe": {"webhook"},
	"square": {"webhookSignature"},
}

// settlementPrefix marks a problem of the settlement kind, distinct from an unchargeable one.
const settlementPrefix = "settlement: "

// settlementProblems says, for each on-processor that can charge, which settlement secret does
// not resolve, with the ref and why. It never makes the processor unavailable: charging still
// works, so these do not count in canCharge and do not raise routing.unchargeable.
func settlementProblems(ctx context.Context, pol routing.Policy) []string {
	var out []string
	for _, r := range pol.Processors {
		if !r.On || processor.DisabledByEnv(processor.ProcessorType(r.Processor)) {
			continue
		}
		for _, a := range pol.Accounts {
			if a.Processor != r.Processor || !a.On || a.Network != "" || len(accountReasons(ctx, a)) > 0 {
				continue
			}
			for _, name := range settlementFields[a.Processor] {
				ref := routing.Get(a.Secrets, name)
				if ref == "" {
					out = append(out, settlementPrefix+a.Processor+" account "+a.ID+" has no KMS path for "+name+": charges are taken but credits cannot be confirmed")
				} else if d := secrets.Diagnose(ctx, ref); d != "" {
					out = append(out, settlementPrefix+a.Processor+" account "+a.ID+" "+name+" at "+ref+": "+d+": charges are taken but credits cannot be confirmed")
				}
			}
		}
	}
	return out
}

// canCharge says some processor that is on has an account that resolves.
func canCharge(ctx context.Context, pol routing.Policy) bool {
	for _, p := range chargeProblems(ctx, pol) {
		if strings.HasPrefix(p, "no processor that is on can charge") {
			return false
		}
	}
	return true
}

// reportUnchargeable says so, loudly, when the platform policy as it runs has a processor
// on and no account for it: an alarm in the log and an event, never a silent boot.
var lastProblems atomic.Value // string

// reportUnchargeableOnChange reports when the set of problems differs from the last one
// reported, and says so once it clears.
func reportUnchargeableOnChange(ctx context.Context, pol routing.Policy) {
	now := strings.Join(append(chargeProblems(ctx, pol), settlementProblems(ctx, pol)...), "\n")
	if prev, _ := lastProblems.Load().(string); prev == now {
		return
	}
	lastProblems.Store(now)
	if now == "" {
		log.Info("routing: the platform policy can charge as configured again")
		return
	}
	reportUnchargeable(ctx, pol)
}

func reportUnchargeable(ctx context.Context, pol routing.Policy) {
	if sp := settlementProblems(ctx, pol); len(sp) > 0 {
		log.Error("ALARM: the platform routing policy cannot confirm what it charges: %s", strings.Join(sp, "; "))
		_, _ = engine.EmitBillingEvent(webhookGuardDB(ctx), "routing.unsettleable", "routing", routingpolicy.Platform, "",
			map[string]interface{}{"problems": sp}, nil)
	}
	problems := chargeProblems(ctx, pol)
	if len(problems) == 0 {
		return
	}
	log.Error("ALARM: the platform routing policy cannot charge as configured: %s", strings.Join(problems, "; "))
	_, _ = engine.EmitBillingEvent(webhookGuardDB(ctx), "routing.unchargeable", "routing", routingpolicy.Platform, "",
		map[string]interface{}{"problems": problems}, nil)
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
		if !a.On || a.Network != "" || a.Processor == "" || processor.DisabledByEnv(processor.ProcessorType(a.Processor)) {
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
		// While anything fails to read, retry on a backoff (1s doubling to runtimeRefresh) so one
		// failed read at boot does not stick; healthy, read every runtimeRefresh.
		wait := time.Second
		t := time.NewTimer(wait)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				next := runtimeRefresh
				pol, v, err := ReadRoutingVersioned(ctx, "")
				if err != nil {
					log.Warn("routing: the platform policy could not be reread: %v", err)
					t.Reset(runtimeRefresh)
					continue
				}
				if int64(v) != appliedVersion.Load() {
					_ = applyRuntime(ctx, pol, v)
				} else {
					// Same policy: read its secrets again, so a secret sealed in KMS after boot
					// (or rotated) reaches the processors without a restart.
					runtimeMu.Lock()
					applyAccounts(ctx, pol)
					configureMPC(ctx, pol)
					configureKraken(ctx, pol)
					runtimeMu.Unlock()
				}
				reportUnchargeableOnChange(ctx, pol)
				if len(chargeProblems(ctx, pol)) > 0 {
					if wait *= 2; wait > runtimeRefresh {
						wait = runtimeRefresh
					}
					next = wait
				} else {
					wait = time.Second
				}
				t.Reset(next)
			}
		}
	}()
	return cancel
}

// currentSeedLevel is what the seed knows how to make today: 1 the processors, the crypto
// deposit accounts, MPC and Kraken; 2 every processor's own account as well (Stripe,
// Square, PayPal and the rest). A policy seeded at a lower level was made by a build that
// did not know these accounts, and is topped up once (topUpSeed).
const currentSeedLevel = 2

// seedRuntime writes the platform policy's first version from the environment the
// deployment was configured with, once, and tops up a policy an older build seeded.
//
// A store that has never held the policy gets the whole seed. A policy seeded at a lower
// level is read once more: each account the environment supplies that the policy lacks is
// added, an account the policy already holds is never touched (an admin may have edited
// it), and the policy's processors and rails are left as they are. After either, the
// policy carries the current level and nothing reads the environment again.
//
// What it carries over: the processors COMMERCE_CHARGE_PROCESSORS named (Stripe alone
// where it was unset and a Stripe key was set, every processor where neither was), the
// crypto deposit accounts CRYPTO_DEPOSIT_* described, the MPC endpoint, an exchange
// account where Kraken keys were set, and every processor's own account. Secrets are never
// copied: an account holds the KMS path they live at.
// holdsAccounts says the stored policy has at least one account: a policy with none, however
// it came to be, is topped up from the environment, since nothing can charge without one.
func holdsAccounts(row *routingpolicy.Row) bool {
	pol, _ := policyOf(row)
	return len(pol.Accounts) > 0
}

func seedRuntime(ctx context.Context) error {
	row, found, err := routingpolicy.Get(ctx, routingpolicy.Platform)
	if err != nil {
		return err
	}
	if found && seeded(row.Rules) && seedLevel(row.Rules) >= currentSeedLevel && holdsAccounts(row) {
		return nil
	}
	pol := routing.Default()
	var version int
	by := "system:env-seed"
	note := "seeded once from the deployment's environment"
	if found {
		pol, _ = policyOf(row)
		version = row.Version
		by = row.By
		{
			// A stored policy (seeded by a build that knew fewer accounts, or written by an admin
			// with none): add the accounts it lacks, and change nothing the admin set.
			scratch := routing.Default()
			scratch.Accounts = nil
			seedFromEnvironment(ctx, &scratch, os.Environ())
			added := 0
			for _, a := range scratch.Accounts {
				if !hasAccount(pol, a) {
					pol.Accounts = append(pol.Accounts, a)
					added++
				}
			}
			note = "topped up from the deployment's environment with the accounts the first seed lacked"
			if added == 0 {
				// Nothing to add: record the level so the environment is not read again, and
				// change no version.
				rows := policyRow(pol, by, version)
				return routingpolicy.Put(ctx, routingpolicy.Platform, rows)
			}
		}
	} else {
		seedFromEnvironment(ctx, &pol, os.Environ())
	}
	pol = pol.Normal()
	rows := policyRow(pol, by, version)
	all, err := routingpolicy.Versions(ctx, routingpolicy.Platform)
	if err != nil {
		return err
	}
	next := 1
	if len(all) > 0 {
		next = all[0].N + 1
	}
	pj, _ := json.Marshal(pol)
	if err := routingpolicy.PutVersion(ctx, routingpolicy.Version{Scope: routingpolicy.Platform, N: next, Policy: string(pj), By: "system:env-seed", Note: note}); err != nil {
		return err
	}
	rows.Version = next
	if err := routingpolicy.Put(ctx, routingpolicy.Platform, rows); err != nil {
		return err
	}
	_, _ = engine.EmitBillingEvent(webhookGuardDB(ctx), "routing.seeded", "routing", routingpolicy.Platform, "",
		map[string]interface{}{"policy": pol, "version": next, "note": note}, nil)
	return nil
}

// hasAccount says pol already holds an account the same as a: the same processor and id
// (and network and asset, for a deposit account).
func hasAccount(pol routing.Policy, a routing.Account) bool {
	for _, x := range pol.Accounts {
		if x.Processor == a.Processor && x.ID == a.ID && x.Network == a.Network && x.Asset == a.Asset {
			return true
		}
	}
	return false
}

func seedLevel(rules string) int {
	var x struct {
		Level int `json:"seedLevel"`
	}
	_ = json.Unmarshal([]byte(rules), &x)
	return x.Level
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
	delete(m, "seedLevel")
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
	case (env["STRIPE_SECRET_KEY"] != "" || env["STRIPE_TEST_SECRET_KEY"] != "") &&
		(env["SQUARE_ACCESS_TOKEN"] != "" || env["SQUARE_SANDBOX_ACCESS_TOKEN"] != ""):
		allowed = processor.ChargeNames("square,stripe")
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
	dropInvalidSeeds(ctx, pol)
	// The default crypto processor an operator named leads the rails for any asset.
	if explicit := strings.ToLower(env["COMMERCE_DEFAULT_CRYPTO_PROCESSOR"]); explicit != "" {
		pol.CryptoRails = append([]routing.CryptoRoute{{Asset: "*", Processor: explicit}}, routing.DefaultCryptoRails()...)
	}
}

// takesCardsAccount says an account of this processor charges cards: the charge processors
// whose accounts the seed makes (Stripe, Square, PayPal). The cards flag is what the routing
// reads to count a processor usable, so a seeded charge account without it is unusable.
func takesCardsAccount(proc string) bool {
	switch proc {
	case "stripe", "square", "paypal":
		return true
	}
	return false
}

// dropInvalidSeeds removes from a seed the crypto accounts (MPC, Kraken) that an admin write
// would refuse, naming each in the log, so a seeded policy always validates. Charge accounts
// stay even when their secrets do not resolve: problems reports those.
func dropInvalidSeeds(ctx context.Context, pol *routing.Policy) {
	kept := pol.Accounts[:0]
	for _, a := range pol.Accounts {
		if a.Processor == string(processor.MPC) || a.Processor == string(processor.Kraken) {
			if why := validateAccounts(ctx, routing.Policy{Accounts: []routing.Account{a}}); why != "" {
				log.Warn("routing seed: leaving out %s account %s: %s", a.Processor, a.ID, why)
				continue
			}
		}
		kept = append(kept, a)
	}
	pol.Accounts = kept
}

// ownValue says the account already holds a value named name.
func ownValue(a routing.Account, name string) (string, bool) {
	if v := routing.Get(a.Secrets, name); v != "" {
		return v, true
	}
	if v := routing.Get(a.Settings, name); v != "" {
		return v, true
	}
	return "", false
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
// (cloud-env-kms-sync, KMS env testnet); Hanzo's own store keeps them at its org root:
// /orgs/hanzo/commerce-secrets for production and /orgs/hanzo/cloud for testnet. The seed
// takes the folder in which the deployment's secrets resolve. Which network this is is the host's KMS reader's
// concern; the path a secret is at is the one that resolves for it.
var kmsFolders = []string{"/commerce-secrets", "/orgs/hanzo/commerce-secrets", "/orgs/hanzo/cloud", "/cloud"}

// kmsRoot is the network's secret root: the first of kmsFolders in which any secret this
// deployment knows of (every processor's, MPC's, Kraken's) resolves. Every ref of a seed is
// derived from it, so one unreadable or unsealed secret cannot land in another network's folder.
func kmsRoot(ctx context.Context) string {
	names := []string{"MPC_API_KEY", "KRAKEN_API_KEY"}
	for _, sp := range processor.Seeds {
		for _, f := range sp.Fields {
			if f.Secret {
				names = append(names, f.Envs...)
			}
		}
	}
	for _, f := range kmsFolders {
		for _, n := range names {
			if secrets.String(ctx, f+"/"+n) != "" {
				return f
			}
		}
	}
	return ""
}

// kmsFolder is the folder holding name for this deployment: where it resolves, else the
// network's root, else the first of kmsFolders (the secret is then not found, and problems say so).
func kmsFolder(ctx context.Context, name string) string {
	for _, f := range kmsFolders {
		if secrets.String(ctx, f+"/"+name) != "" {
			return f
		}
	}
	if root := kmsRoot(ctx); root != "" {
		return root
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
		for _, f := range sp.Shared {
			if !sp.SharedAllowed(env) {
				break
			}
			if _, own := ownValue(a, f.Name); own {
				continue
			}
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
		a.Cards = takesCardsAccount(sp.Processor)
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

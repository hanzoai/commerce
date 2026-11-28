package billing

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/hanzoai/commerce/billing/depositledger"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/cryptopaymentintent"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/routingpolicy"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/payment/providers/kraken"
	"github.com/hanzoai/commerce/payment/routing"
	"github.com/hanzoai/commerce/secrets"
	"github.com/hanzoai/commerce/thirdparty/mpc"
	"github.com/hanzoai/commerce/util/test/ae"
)

// mapSecrets is a KMS holding exactly the paths it is given.
type mapSecrets map[string]string

func (m mapSecrets) GetSecret(_ context.Context, ref string) ([]byte, error) {
	if v, ok := m[ref]; ok {
		return []byte(v), nil
	}
	return nil, errors.New("no such secret")
}

func useSecrets(t *testing.T, m mapSecrets) {
	t.Helper()
	old := secrets.Get()
	secrets.Set(m)
	t.Cleanup(func() { secrets.Set(old) })
}

// installRuntime puts the platform policy under the processors, as boot does, and takes it
// away again at the end of the test.
func installRuntime(t *testing.T, ctx context.Context) {
	t.Helper()
	if err := InstallRuntime(ctx); err != nil {
		t.Fatalf("InstallRuntime: %v", err)
	}
	t.Cleanup(func() { applied.Store(nil); processor.SetChargePolicy(nil) })
}

// Switching a processor off in the policy refuses the next attempt on a registry built
// before the change, with no restart; switching it on takes it again.
func TestSwitchingAProcessorOffInThePolicyRefusesTheNextAttempt(t *testing.T) {
	ctx := heldStore(t)
	installRuntime(t, ctx)
	reg := processor.NewRegistry(processor.DefaultConfig())
	reg.Register(newRail(processor.Square, true))
	reg.Register(newRail(processor.Stripe, true))

	if _, err := reg.Get(processor.Square); err != nil {
		t.Fatalf("Square is on by default: %v", err)
	}
	off := routing.Default()
	setOn(&off, "square", false)
	if _, f := WriteRouting(ctx, "", off, "z@hanzo.ai", "square off"); f != nil {
		t.Fatal(f.Err)
	}
	if _, err := reg.Get(processor.Square); err == nil {
		t.Fatal("Square was switched off in the policy and the next attempt still took it")
	}
	if _, err := reg.Get(processor.Stripe); err != nil {
		t.Fatalf("Stripe is still on: %v", err)
	}
	if _, f := WriteRouting(ctx, "", routing.Default(), "z@hanzo.ai", "square on"); f != nil {
		t.Fatal(f.Err)
	}
	if _, err := reg.Get(processor.Square); err != nil {
		t.Fatalf("Square back on and still refused: %v", err)
	}
}

// A deposit account added in the policy is watched and offered at once; removed, the
// watcher stops reading it.
func TestAddingADepositAccountStartsTheWatcherAndRemovingItStopsIt(t *testing.T) {
	ctx := heldStore(t)
	svc := installWatcher(t, nil, true)
	installRuntime(t, ctx)
	if len(watchedAssets()) != 0 {
		t.Fatal("nothing is configured and something is watched")
	}
	withAccount := routing.Default()
	withAccount.Accounts = accountsOf(baseUSDCEnv())
	if _, f := WriteRouting(ctx, "", withAccount, "z@hanzo.ai", "watch base usdc"); f != nil {
		t.Fatalf("add the account: %v", f.Err)
	}
	got := watchedAssets()
	if len(got) != 1 || got[0].Chain != "base" || got[0].Token != "usdc" || !svc.Running() {
		t.Fatalf("watched = %+v (running %v); want base/usdc watched and offered", got, svc.Running())
	}
	if _, f := WriteRouting(ctx, "", routing.Default(), "z@hanzo.ai", "stop"); f != nil {
		t.Fatal(f.Err)
	}
	if len(watchedAssets()) != 0 {
		t.Fatalf("the account was removed and %+v is still watched", watchedAssets())
	}
}

// An account's secretRef is a KMS path that must resolve when the policy is written, and
// its endpoint must be https; the value is never stored.
func TestASecretRefThatDoesNotResolveIsRefused(t *testing.T) {
	ctx := heldStore(t)
	installRuntime(t, ctx)
	useSecrets(t, mapSecrets{"/commerce-secrets/MPC_API_KEY": "s3cret"})
	mk := func(ref, endpoint string) routing.Policy {
		p := routing.Default()
		setOn(&p, "mpc", true)
		p.Accounts = []routing.Account{{Processor: "mpc", ID: "service", Endpoint: endpoint, SecretRef: ref, On: true}}
		return p
	}
	if _, f := WriteRouting(ctx, "", mk("/commerce-secrets/NOT_THERE", "https://mpc.example"), "z", ""); f == nil || f.Status != 400 {
		t.Fatalf("a secretRef that does not resolve was accepted: %v", f)
	}
	if _, f := WriteRouting(ctx, "", mk("MPC_API_KEY", "https://mpc.example"), "z", ""); f == nil {
		t.Fatal("a secretRef that is not a KMS path was accepted")
	}
	if _, f := WriteRouting(ctx, "", mk("/commerce-secrets/MPC_API_KEY", "http://mpc.example"), "z", ""); f == nil {
		t.Fatal("an endpoint that is not https was accepted")
	}
	p, f := WriteRouting(ctx, "", mk("/commerce-secrets/MPC_API_KEY", "https://mpc.example"), "z", "")
	if f != nil {
		t.Fatalf("a resolving ref and an https endpoint were refused: %v", f.Err)
	}
	row, _, _ := routingpolicy.Get(ctx, routingpolicy.Platform)
	if p.Accounts[0].SecretRef != "/commerce-secrets/MPC_API_KEY" || containsSecret(row.Rules, "s3cret") {
		t.Fatalf("the policy stores the secret, not its path: %s", row.Rules)
	}
}

func containsSecret(rules, v string) bool { return len(v) > 0 && strings.Contains(rules, v) }

// The first run seeds the platform policy from the environment, once; after that the
// environment is not read, and an admin's change is not overwritten by a restart.
func TestTheEnvironmentSeedsThePolicyOnceAndIsNeverReadAgain(t *testing.T) {
	ctx := heldStore(t)
	t.Setenv("COMMERCE_CHARGE_PROCESSORS", "stripe,kraken")
	t.Setenv("CRYPTO_DEPOSIT_RPC_BASE", "https://mainnet.base.org")
	t.Setenv("CRYPTO_DEPOSIT_TOKEN_BASE_USDC", "0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913")
	t.Setenv("MPC_ENDPOINT", "https://mpc.example")
	t.Setenv("MPC_API_KEY", "x")
	t.Setenv("MPC_KMS_ENDPOINT", "https://kms.example")
	t.Setenv("MPC_ZAP_ADDR", "mpc.svc:9805")
	t.Setenv("MPC_WEBHOOK_SECRET", "wh")
	useSecrets(t, mapSecrets{"/commerce-secrets/MPC_API_KEY": "x", "/commerce-secrets/MPC_WEBHOOK_SECRET": "wh"})
	installRuntime(t, ctx)

	pol, v, err := ReadRoutingVersioned(ctx, "")
	if err != nil || v != 1 {
		t.Fatalf("seeded policy = v%d (%v)", v, err)
	}
	on := map[string]bool{}
	for _, r := range pol.Processors {
		on[r.Processor] = r.On
	}
	if !on["stripe"] || !on["kraken"] || on["square"] || on["paypal"] || on["mpc"] {
		t.Fatalf("processors = %+v; want exactly stripe and kraken on", pol.Processors)
	}
	var base, mpcSvc *routing.Account
	for i, a := range pol.Accounts {
		switch {
		case a.Network == "base" && a.Asset == "usdc":
			base = &pol.Accounts[i]
		case a.Processor == "mpc" && a.Network == "":
			mpcSvc = &pol.Accounts[i]
		}
	}
	if base == nil || base.Endpoint != "https://mainnet.base.org" || base.Token != "0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913" ||
		mpcSvc == nil || mpcSvc.Endpoint != "https://mpc.example" || mpcSvc.SecretRef != "/commerce-secrets/MPC_API_KEY" ||
		routing.Get(mpcSvc.Endpoints, "kms") != "https://kms.example" || routing.Get(mpcSvc.Endpoints, "zap") != "mpc.svc:9805" ||
		routing.Get(mpcSvc.Secrets, "webhook") != "/commerce-secrets/MPC_WEBHOOK_SECRET" {
		t.Fatalf("accounts = %+v", pol.Accounts)
	}

	// A restart with different environment changes nothing: it is read once.
	t.Setenv("COMMERCE_CHARGE_PROCESSORS", "*")
	t.Setenv("MPC_ENDPOINT", "https://other.example")
	if err := seedRuntime(ctx); err != nil {
		t.Fatal(err)
	}
	again, v2, _ := ReadRoutingVersioned(ctx, "")
	if v2 != 1 || len(again.Accounts) != len(pol.Accounts) || !again.On(processor.Stripe) || again.On(processor.Square) {
		t.Fatalf("the environment was read again: v%d %+v", v2, again.Processors)
	}
	// And an admin's change survives it.
	changed := again
	setOn(&changed, "square", true)
	if _, f := WriteRouting(ctx, "", changed, "z@hanzo.ai", "square on"); f != nil {
		t.Fatal(f.Err)
	}
	if err := seedRuntime(ctx); err != nil {
		t.Fatal(err)
	}
	if final, _, _ := ReadRoutingVersioned(ctx, ""); !final.On(processor.Square) {
		t.Fatal("a restart seeded over the admin's change")
	}
}

// USDC goes to Stripe and BTC to Kraken, both by the policy's rails; flipping the order
// there takes effect on the next attempt.
func TestCryptoRailsFollowThePolicyAndItsOrder(t *testing.T) {
	ctx := heldStore(t)
	installWatcher(t, baseUSDCEnv(), true)
	installRuntime(t, ctx)
	stripe := newRail(processor.Stripe, true)
	reg := processor.NewRegistry(processor.DefaultConfig())
	reg.Register(stripe)

	pol := routing.Default()
	setOn(&pol, "kraken", true)
	pol.Accounts = accountsOf(baseUSDCEnv())
	// Crypto is Stripe stablecoins by default; the admin adds Kraken for the coins Stripe lacks.
	pol.CryptoRails = append(pol.CryptoRails, routing.CryptoRoute{Asset: "btc", Processor: "kraken"})
	if _, f := WriteRouting(ctx, "", pol, "z", "kraken on"); f != nil {
		t.Fatalf("policy: %v", f.Err)
	}
	route := func() map[string]string {
		got := map[string]string{}
		for _, r := range cryptoRoutesFor(ctx, reg, mustRead(t, ctx)) {
			got[r.Asset] = r.Processor
		}
		return got
	}
	if r := route(); r["usdc"] != "stripe" || r["btc"] != "kraken" {
		t.Fatalf("routes = %v; want usdc on stripe and btc on kraken", r)
	}
	// The admin puts Kraken ahead of Stripe for USDC.
	flipped := mustRead(t, ctx)
	flipped.CryptoRails = append([]routing.CryptoRoute{{Asset: "usdc", Processor: "kraken"}}, flipped.CryptoRails...)
	if _, f := WriteRouting(ctx, "", flipped, "z", "kraken first for usdc"); f != nil {
		t.Fatal(f.Err)
	}
	if r := route(); r["usdc"] != "kraken" {
		t.Fatalf("routes after the flip = %v; want usdc on kraken", r)
	}
	// With Stripe's crypto not offered the Payment Element is told to leave it out.
	if mustRead(t, ctx).StripeCrypto() {
		t.Fatal("Stripe crypto is offered while Kraken leads for USDC")
	}
	found := false
	for _, m := range mustRead(t, ctx).Off() {
		found = found || m == "crypto"
	}
	if !found {
		t.Fatal("the Payment Element would still offer crypto")
	}
}

func mustRead(t *testing.T, ctx context.Context) routing.Policy {
	t.Helper()
	p, err := ReadRouting(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// A Kraken deposit address is bound to exactly one open payment: when the exchange hands
// the same address back for a second payment, that payment is refused, never given it.
func TestAKrakenAddressIsNeverSharedByTwoOpenPayments(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	installWatcher(t, baseUSDCEnv(), true)
	kp := routing.Default()
	setOn(&kp, "kraken", true)
	kp.Accounts = accountsOf(baseUSDCEnv())
	kp.CryptoRails = []routing.CryptoRoute{{Asset: "usdc", Processor: "kraken"}}
	if _, f := WriteRouting(c, "", kp, "test", ""); f != nil {
		t.Fatalf("policy: %v", f.Err)
	}

	var calls int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/0/private/DepositMethods":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": []string{}, "result": []kraken.DepositMethod{{Method: "USDC (Base)", GenAddress: true}}})
		case "/0/private/DepositAddresses":
			atomic.AddInt64(&calls, 1)
			// The exchange runs out of fresh addresses and hands back the one it already issued.
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": []string{}, "result": []kraken.DepositAddress{{Address: fmt.Sprintf("0x%040x", 7), ExpireTm: "0", New: false}}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	secret := base64.StdEncoding.EncodeToString([]byte("test-kraken-secret-key-for-hmac-sha512"))
	processor.Register(kraken.NewProcessor(kraken.Config{APIKey: "k", APISecret: secret, APIURL: srv.URL}))
	t.Cleanup(func() { processor.Register(kraken.NewProcessor(kraken.Config{})) })

	org := organization.New(datastore.New(c))
	org.Name = "acme"
	if err := org.Create(); err != nil {
		t.Fatal(err)
	}
	first, err := CreateCryptoDeposit(context.Background(), org, "acme/alice", "base", "usdc", 2000)
	if err != nil || first.DepositAddress == "" {
		t.Fatalf("the first payment: %v", err)
	}
	second, err := CreateCryptoDeposit(context.Background(), org, "acme/bob", "base", "usdc", 2000)
	if err == nil {
		t.Fatalf("a second open payment was given the address %q the first holds", second.DepositAddress)
	}
	// Once the first payment is no longer open, the address is free to be bound again.
	row := cryptoIntentByAddress(t, org, c, first.DepositAddress)
	in := cryptopaymentintent.New(datastore.New(org.Namespaced(c)))
	if err := in.GetById(row.Id()); err != nil {
		t.Fatal(err)
	}
	in.Status = cryptopaymentintent.Expired
	if err := in.Update(); err != nil {
		t.Fatal(err)
	}
	if third, err := CreateCryptoDeposit(context.Background(), org, "acme/bob", "base", "usdc", 2000); err != nil || third.DepositAddress != first.DepositAddress {
		t.Fatalf("after the first closed: %v %+v", err, third)
	}
}

var _ = depositledger.Default

func cryptoIntentByAddress(t *testing.T, org *organization.Organization, ctx context.Context, address string) *cryptopaymentintent.CryptoPaymentIntent {
	t.Helper()
	rows := make([]*cryptopaymentintent.CryptoPaymentIntent, 0)
	if _, err := cryptopaymentintent.Query(datastore.New(org.Namespaced(ctx))).Filter("DepositAddress=", address).GetAll(&rows); err != nil || len(rows) == 0 {
		t.Fatalf("no intent holds %s (%v)", address, err)
	}
	return rows[0]
}

// The mpc account in the policy configures the rail live: its endpoint, its KMS and ZAP
// endpoints, and the webhook secret read from KMS; removed, the rail is off.
func TestTheMPCAccountConfiguresTheRailAtRuntime(t *testing.T) {
	ctx := heldStore(t)
	installRuntime(t, ctx)
	useSecrets(t, mapSecrets{"/commerce-secrets/MPC_API_KEY": "k", "/commerce-secrets/MPC_WEBHOOK_SECRET": "wh"})
	p, err := processor.Global().Registered(processor.MPC)
	if err != nil {
		t.Skip("mpc is not registered in this build")
	}
	rail := p.(*mpc.MPCProcessor)
	if rail.ReceivesWebhooks() {
		t.Fatal("the rail takes webhooks before any account names a secret")
	}
	pol := routing.Default()
	setOn(&pol, "mpc", true)
	pol.Accounts = []routing.Account{{Processor: "mpc", ID: "service", Endpoint: "https://mpc.example", On: true, SecretRef: "/commerce-secrets/MPC_API_KEY",
		Endpoints: []routing.Named{{Name: "kms", Value: "https://kms.example"}, {Name: "zap", Value: "mpc.svc:9805"}},
		Secrets:   []routing.Named{{Name: "webhook", Value: "/commerce-secrets/MPC_WEBHOOK_SECRET"}}}}
	if _, f := WriteRouting(ctx, "", pol, "z", ""); f != nil {
		t.Fatalf("policy: %v", f.Err)
	}
	if !rail.ReceivesWebhooks() {
		t.Fatal("the webhook secret in the account did not reach the rail")
	}
	// A webhook secret that does not resolve is refused.
	bad := pol
	bad.Accounts = []routing.Account{pol.Accounts[0]}
	bad.Accounts[0].Secrets = []routing.Named{{Name: "webhook", Value: "/commerce-secrets/NOPE"}}
	if _, f := WriteRouting(ctx, "", bad, "z", ""); f == nil {
		t.Fatal("an unresolvable webhook secret was accepted")
	}
	if _, f := WriteRouting(ctx, "", routing.Default(), "z", ""); f != nil {
		t.Fatal(f.Err)
	}
	if rail.ReceivesWebhooks() {
		t.Fatal("the account was removed and the rail still takes webhooks")
	}
}

// A processor's keys come from its account in the policy, read from KMS when the policy is
// applied: the secret key reaches the provider, which is configured; the account removed,
// it is not. No environment is involved, and the policy stores the path, never the key.
func TestAProcessorIsConfiguredFromItsAccountAndTheSecretStaysInKMS(t *testing.T) {
	ctx := heldStore(t)
	installRuntime(t, ctx)
	useSecrets(t, mapSecrets{"/commerce-secrets/STRIPE_SECRET_KEY": "sk_live_FROMKMS", "/commerce-secrets/STRIPE_WEBHOOK_SECRET": "whsec_x"})
	p, err := processor.Global().Registered(processor.Stripe)
	if err != nil {
		t.Fatal(err)
	}
	if p.IsAvailable(ctx) {
		t.Fatal("Stripe is available before any account holds its key")
	}
	pol := routing.Default()
	pol.Accounts = []routing.Account{{Processor: "stripe", ID: "live", On: true, Cards: true,
		Settings: []routing.Named{{Name: "publishableKey", Value: "pk_live_PUB"}},
		Secrets:  []routing.Named{{Name: "secretKey", Value: "/commerce-secrets/STRIPE_SECRET_KEY"}, {Name: "webhook", Value: "/commerce-secrets/STRIPE_WEBHOOK_SECRET"}}}}
	saved, f := WriteRouting(ctx, "", pol, "z", "stripe keys")
	if f != nil {
		t.Fatalf("policy: %v", f.Err)
	}
	if !p.IsAvailable(ctx) || !stripeReceivesWebhooks(p) {
		t.Fatal("the key and the webhook secret in the account did not reach Stripe")
	}
	row, _, _ := routingpolicy.Get(ctx, routingpolicy.Platform)
	if strings.Contains(row.Rules, "sk_live_FROMKMS") || strings.Contains(row.Rules, "whsec_x") || saved.Accounts[0].Secrets[0].Value != "/commerce-secrets/STRIPE_SECRET_KEY" {
		t.Fatalf("the policy stores a secret, not its path: %s", row.Rules)
	}
	// A key that is not in KMS is refused, and the running configuration is untouched.
	bad := pol
	bad.Accounts = []routing.Account{pol.Accounts[0]}
	bad.Accounts[0].Secrets = []routing.Named{{Name: "secretKey", Value: "/commerce-secrets/NOT_THERE"}}
	if _, f := WriteRouting(ctx, "", bad, "z", ""); f == nil || !p.IsAvailable(ctx) {
		t.Fatalf("an unresolvable key was accepted or broke the running one: %v", f)
	}
	// The last account cannot be dropped while a processor that needs one is on: that is a
	// deploy with no way to charge a card.
	if _, f := WriteRouting(ctx, "", routing.Default(), "z", "no stripe account"); f == nil || f.Status != 400 || !p.IsAvailable(ctx) {
		t.Fatalf("a save that left nothing able to charge was accepted: %v", f)
	}
	crypto := routing.Default()
	for _, n := range []string{"stripe", "square", "paypal"} {
		setOn(&crypto, n, false)
	}
	setOn(&crypto, "kraken", true)
	crypto.Processors = append(crypto.Processors, routing.ProcessorRule{Processor: "adyen", On: true}) // takes cards, needs no account here
	if _, f := WriteRouting(ctx, "", crypto, "z", "account-backed processors off, no account"); f != nil {
		t.Fatalf("a policy with every account-backed processor off was refused: %v", f.Err)
	}
	if p.IsAvailable(ctx) {
		t.Fatal("the account was removed and Stripe still holds its key")
	}
}

func stripeReceivesWebhooks(p processor.PaymentProcessor) bool {
	r, ok := p.(interface{ ReceivesWebhooks() bool })
	return ok && r.ReceivesWebhooks()
}

// The first run turns the environment's processor settings into accounts: plain values as
// settings, each secret as the KMS path it lives at, and Square's and PayPal's live and
// sandbox sets side by side with the named one marked as the deployment's.
func TestTheSeedTurnsProcessorEnvironmentIntoAccountsWithSecretPaths(t *testing.T) {
	pol := routing.Default()
	seedFromEnvironment(context.Background(), &pol, []string{
		"STRIPE_SECRET_KEY=sk_live_X", "STRIPE_PUBLISHABLE_KEY=pk_live_X", "STRIPE_TEST_SECRET_KEY=sk_test_X",
		"SQUARE_ENVIRONMENT=sandbox", "SQUARE_ACCESS_TOKEN=tok", "SQUARE_LOCATION_ID=LOC", "SQUARE_SANDBOX_ACCESS_TOKEN=sbtok", "SQUARE_SANDBOX_LOCATION_ID=SBLOC",
		"PAYPAL_CLIENT_ID=pp", "PAYPAL_CLIENT_SECRET=pps", "MOONPAY_API_KEY=mp", "OPENNODE_API_KEY=on", "OPENNODE_ENVIRONMENT=live",
		"SOLANA_RPC_URL=https://rpc.example", "SOLANA_RECIPIENT_ADDRESS=Addr", "KRAKEN_DEPOSIT_METHOD_BASE_USDC=USDC (Base)",
	})
	find := func(proc, id string) *routing.Account {
		for i, a := range pol.Accounts {
			if a.Processor == proc && a.ID == id {
				return &pol.Accounts[i]
			}
		}
		return nil
	}
	set := func(l []routing.Named, name string) string { return routing.Get(l, name) }
	stripe := find("stripe", "live")
	if stripe == nil || set(stripe.Secrets, "secretKey") != "/commerce-secrets/STRIPE_SECRET_KEY" || set(stripe.Settings, "publishableKey") != "pk_live_X" {
		t.Fatalf("stripe live = %+v", stripe)
	}
	if a := find("stripe", "test"); a == nil || set(a.Secrets, "secretKey") != "/commerce-secrets/STRIPE_TEST_SECRET_KEY" {
		t.Fatalf("stripe test = %+v", a)
	}
	live, sandbox := find("square", "live"), find("square", "sandbox")
	if live == nil || sandbox == nil || set(live.Settings, "deployment") == "true" || set(sandbox.Settings, "deployment") != "true" ||
		set(sandbox.Secrets, "accessToken") != "/commerce-secrets/SQUARE_SANDBOX_ACCESS_TOKEN" || set(sandbox.Settings, "locationId") != "SBLOC" {
		t.Fatalf("square = %+v %+v; want both sets, the sandbox marked as the deployment's", live, sandbox)
	}
	if a := find("paypal", "live"); a == nil || set(a.Settings, "clientId") != "pp" || set(a.Secrets, "clientSecret") != "/commerce-secrets/PAYPAL_CLIENT_SECRET" || set(a.Settings, "deployment") != "true" {
		t.Fatalf("paypal = %+v", a)
	}
	if a := find("opennode", "default"); a == nil || set(a.Settings, "environment") != "live" || set(a.Secrets, "apiKey") != "/commerce-secrets/OPENNODE_API_KEY" {
		t.Fatalf("opennode = %+v", a)
	}
	if a := find("solanapay", "default"); a == nil || set(a.Settings, "rpcUrl") != "https://rpc.example" || set(a.Settings, "recipient") != "Addr" {
		t.Fatalf("solanapay = %+v", a)
	}
	if a := find("kraken", "exchange"); a == nil || set(a.Settings, "depositMethod.base.usdc") != "USDC (Base)" {
		t.Fatalf("kraken = %+v", a)
	}
	for _, a := range pol.Accounts {
		for _, n := range append(append([]routing.Named{}, a.Settings...), a.Endpoints...) {
			if strings.HasPrefix(n.Value, "sk_") || n.Value == "tok" || n.Value == "pps" {
				t.Errorf("account %s/%s holds a secret value as a setting: %+v", a.Processor, a.ID, n)
			}
		}
	}
}

// A secret is named at the path it resolves at on this deployment: production's are under
// /commerce-secrets, testnet's under /cloud (the universe KMS syncs), and the seed finds
// whichever holds them.
func TestTheSeedNamesEachSecretAtThePathThatResolvesOnThisDeployment(t *testing.T) {
	useSecrets(t, mapSecrets{
		"/cloud/STRIPE_TEST_SECRET_KEY": "x", "/cloud/SQUARE_SANDBOX_ACCESS_TOKEN": "y", "/cloud/PAYPAL_SANDBOX_CLIENT_SECRET": "z",
		"/commerce-secrets/KRAKEN_API_KEY": "k", "/commerce-secrets/KRAKEN_API_SECRET": "s",
	})
	pol := routing.Default()
	seedFromEnvironment(context.Background(), &pol, []string{
		"STRIPE_TEST_SECRET_KEY=a", "SQUARE_SANDBOX_ACCESS_TOKEN=b", "PAYPAL_SANDBOX_CLIENT_SECRET=c", "PAYPAL_SANDBOX_CLIENT_ID=id",
		"KRAKEN_API_KEY=k", "KRAKEN_API_SECRET=s",
	})
	want := map[string]string{"stripe/test/secretKey": "/cloud/STRIPE_TEST_SECRET_KEY", "square/sandbox/accessToken": "/cloud/SQUARE_SANDBOX_ACCESS_TOKEN", "paypal/sandbox/clientSecret": "/cloud/PAYPAL_SANDBOX_CLIENT_SECRET"}
	got := map[string]string{}
	for _, a := range pol.Accounts {
		for _, n := range a.Secrets {
			got[a.Processor+"/"+a.ID+"/"+n.Name] = n.Value
		}
		if a.Processor == "kraken" && a.SecretRef != "/commerce-secrets" {
			t.Errorf("kraken folder = %q, want /commerce-secrets", a.SecretRef)
		}
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}
}

// A policy a build seeded before it knew the processors' own accounts (only crypto and
// MPC/Kraken) is topped up once with the accounts the environment supplies: the ones it
// lacks are added, one the admin edited is left as the admin made it, nothing else moves,
// and a second run changes nothing.
func TestACryptoOnlySeededPolicyIsToppedUpWithTheChargeAccounts(t *testing.T) {
	ctx := heldStore(t)
	t.Setenv("STRIPE_SECRET_KEY", "sk_live_env")
	t.Setenv("STRIPE_PUBLISHABLE_KEY", "pk_live_ENV")
	t.Setenv("SQUARE_ACCESS_TOKEN", "tok")
	t.Setenv("SQUARE_LOCATION_ID", "LOC")
	t.Setenv("PAYPAL_CLIENT_ID", "pp")
	t.Setenv("PAYPAL_CLIENT_SECRET", "pps")
	t.Setenv("COMMERCE_CHARGE_PROCESSORS", "stripe,square,paypal")
	useSecrets(t, mapSecrets{"/commerce-secrets/STRIPE_SECRET_KEY": "x", "/commerce-secrets/SQUARE_ACCESS_TOKEN": "x", "/commerce-secrets/PAYPAL_CLIENT_SECRET": "x"})

	// What v1.50.221 left: seeded, with the crypto accounts only.
	old := routing.Default()
	for _, n := range []string{"stripe", "square", "paypal"} {
		setOn(&old, n, true)
	}
	old.Accounts = []routing.Account{
		{Processor: "mpc", ID: "service", Endpoint: "https://mpc.example", On: true},
		{Processor: "kraken", ID: "exchange", On: true, SecretRef: "/commerce-secrets"},
		// An account the admin already made and edited by hand.
		{Processor: "stripe", ID: "live", On: true, Cards: true, Settings: []routing.Named{{Name: "publishableKey", Value: "pk_live_ADMIN"}},
			Secrets: []routing.Named{{Name: "secretKey", Value: "/commerce-secrets/STRIPE_SECRET_KEY"}}},
	}
	row := policyRow(old.Normal(), "z@hanzo.ai", 3)
	var m map[string]json.RawMessage
	_ = json.Unmarshal([]byte(row.Rules), &m)
	delete(m, "seedLevel")
	b, _ := json.Marshal(m)
	row.Rules = string(b)
	if err := routingpolicy.Put(ctx, routingpolicy.Platform, row); err != nil {
		t.Fatal(err)
	}
	if err := routingpolicy.PutVersion(ctx, routingpolicy.Version{Scope: routingpolicy.Platform, N: 3, Policy: "{}", By: "system:env-seed"}); err != nil {
		t.Fatal(err)
	}
	before, _, _ := ReadRoutingVersioned(ctx, "")
	if got := chargeProblems(ctx, before); len(got) != 2 {
		t.Fatalf("the crypto-only policy reports %v; want square and paypal holding no account", got)
	}

	if err := seedRuntime(ctx); err != nil {
		t.Fatal(err)
	}
	after, v, _ := ReadRoutingVersioned(ctx, "")
	has := func(proc, id string) *routing.Account {
		for i, a := range after.Accounts {
			if a.Processor == proc && a.ID == id {
				return &after.Accounts[i]
			}
		}
		return nil
	}
	if has("square", "live") == nil || has("paypal", "live") == nil || has("mpc", "service") == nil || has("kraken", "exchange") == nil {
		t.Fatalf("accounts after the top-up = %+v; want square, paypal added and mpc, kraken kept", after.Accounts)
	}
	if st := has("stripe", "live"); st == nil || routing.Get(st.Settings, "publishableKey") != "pk_live_ADMIN" {
		t.Fatalf("the admin's stripe account was overwritten: %+v", st)
	}
	if v != 4 {
		t.Fatalf("version = %d; want 4 (one top-up)", v)
	}
	if got := chargeProblems(ctx, after); len(got) != 0 {
		t.Fatalf("after the top-up: %v", got)
	}
	// Idempotent: a second boot adds nothing and moves no version.
	if err := seedRuntime(ctx); err != nil {
		t.Fatal(err)
	}
	again, v2, _ := ReadRoutingVersioned(ctx, "")
	if v2 != 4 || len(again.Accounts) != len(after.Accounts) {
		t.Fatalf("a second boot changed the policy: v%d, %d accounts", v2, len(again.Accounts))
	}
}

// A platform policy with a processor on and no account to charge with is said loudly, and a
// save cannot take a charging policy there.
func TestAPolicyWithNothingToChargeWithIsReportedAndCannotBeSavedInto(t *testing.T) {
	ctx := context.Background()
	useSecrets(t, mapSecrets{"/commerce-secrets/STRIPE_SECRET_KEY": "x"})
	pol := routing.Default()
	if got := chargeProblems(ctx, pol); len(got) != 4 || canCharge(ctx, pol) {
		t.Fatalf("problems for a policy with cards on and no account = %v", got)
	}
	pol.Accounts = []routing.Account{{Processor: "stripe", ID: "live", On: true, Cards: true, Secrets: []routing.Named{{Name: "secretKey", Value: "/commerce-secrets/STRIPE_SECRET_KEY"}}}}
	if got := chargeProblems(ctx, pol); len(got) != 2 || !canCharge(ctx, pol) {
		t.Fatalf("with a stripe account: %v (square and paypal still hold none, but cards can charge)", got)
	}
	crypto := routing.Default()
	for _, n := range []string{"stripe", "square", "paypal"} {
		setOn(&crypto, n, false)
	}
	if got := chargeProblems(ctx, crypto); len(got) != 0 {
		t.Fatalf("a policy with every account-backed processor off reports %v", got)
	}
}

// The problem list keys on what RESOLVES, not on what is listed: an account whose secret
// paths are not in KMS leaves its processor unable to charge, and the report names the path
// and why (not found, permission denied, empty) and never a value.
func TestProblemsKeyOnWhatResolvesAndNameTheFailingPathAndWhy(t *testing.T) {
	ctx := context.Background()
	pol := routing.Default()
	pol.Accounts = []routing.Account{
		{Processor: "stripe", ID: "live", On: true, Cards: true, Secrets: []routing.Named{{Name: "secretKey", Value: "/commerce-secrets/STRIPE_SECRET_KEY"}}},
		{Processor: "square", ID: "live", On: true, Cards: true, Settings: []routing.Named{{Name: "locationId", Value: "LOC"}}, Secrets: []routing.Named{{Name: "accessToken", Value: "/commerce-secrets/SQUARE_ACCESS_TOKEN"}}},
		{Processor: "paypal", ID: "live", On: true, Cards: true, Settings: []routing.Named{{Name: "clientId", Value: "pp"}}, Secrets: []routing.Named{{Name: "clientSecret", Value: "/commerce-secrets/PAYPAL_CLIENT_SECRET"}}},
	}
	// None resolves: each reader answer is a different reason.
	secrets.Set(failingSecrets{"/commerce-secrets/STRIPE_SECRET_KEY": secrets.ErrNotFound, "/commerce-secrets/SQUARE_ACCESS_TOKEN": secrets.ErrDenied, "/commerce-secrets/PAYPAL_CLIENT_SECRET": nil})
	t.Cleanup(func() { secrets.Set(nil) })
	got := strings.Join(chargeProblems(ctx, pol), "\n")
	for _, want := range []string{
		"stripe is on and no account of it is able to charge (unchargeable): account live secret secretKey at /commerce-secrets/STRIPE_SECRET_KEY: not found",
		"square is on and no account of it is able to charge (unchargeable): account live secret accessToken at /commerce-secrets/SQUARE_ACCESS_TOKEN: permission denied",
		"paypal is on and no account of it is able to charge (unchargeable): account live secret clientSecret at /commerce-secrets/PAYPAL_CLIENT_SECRET: empty value",
		"no processor that is on can charge",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("problems lack %q:\n%s", want, got)
		}
	}
	if canCharge(ctx, pol) {
		t.Fatal("a policy whose every secret fails to resolve reports it can charge")
	}
	// One resolves: the others are still named, but cards can charge.
	secrets.Set(mapSecrets{"/commerce-secrets/STRIPE_SECRET_KEY": "v"})
	if !canCharge(ctx, pol) || len(chargeProblems(ctx, pol)) != 2 {
		t.Fatalf("with Stripe's key resolving: %v", chargeProblems(ctx, pol))
	}
	// With no reader installed nothing resolves, and it says so.
	secrets.Set(nil)
	if !strings.Contains(strings.Join(chargeProblems(ctx, pol), "\n"), "no secret reader installed") {
		t.Fatal("no reader, and the report does not say so")
	}
}

// failingSecrets answers each path with the error it is given (nil: an empty value).
type failingSecrets map[string]error

func (f failingSecrets) GetSecret(_ context.Context, ref string) ([]byte, error) {
	if err, ok := f[ref]; ok && err != nil {
		return nil, err
	}
	return nil, nil
}

// flakySecrets fails the first n reads of each path with err, then answers.
type flakySecrets struct {
	mu    sync.Mutex
	fails int
	err   error
	seen  map[string]int
}

func (f *flakySecrets) GetSecret(_ context.Context, ref string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.seen == nil {
		f.seen = map[string]int{}
	}
	f.seen[ref]++
	if f.seen[ref] <= f.fails {
		return nil, f.err
	}
	return []byte("value"), nil
}

// A reader that fails once and then succeeds ends available once the policy is re-read;
// one that always fails ends in problems with the ref and the reason, and never a value.
func TestAReaderThatFailsOnceEndsAvailableAndOneThatAlwaysFailsEndsInProblems(t *testing.T) {
	ctx := context.Background()
	pol := routing.Default()
	pol.Accounts = []routing.Account{{Processor: "stripe", ID: "live", On: true, Cards: true, Secrets: []routing.Named{{Name: "secretKey", Value: "/commerce-secrets/STRIPE_SECRET_KEY"}}}}
	for _, p := range []string{"square", "paypal"} {
		for i := range pol.Processors {
			if pol.Processors[i].Processor == p {
				pol.Processors[i].On = false
			}
		}
	}
	t.Cleanup(func() { secrets.Set(nil); processor.SetAccounts(processor.Stripe, nil) })

	key := func() string { a, _ := processor.FirstAccount(processor.Stripe); return a.Secret("secretKey") }

	// Fails once, then succeeds: unavailable after the boot read, available after the re-read.
	secrets.Set(&flakySecrets{fails: 1, err: secrets.ErrTimeout})
	applyAccounts(ctx, pol)
	if key() != "" || secrets.Failure("/commerce-secrets/STRIPE_SECRET_KEY") != "timeout" {
		t.Fatalf("the boot read should have failed and been recorded: key=%q failure=%q", key(), secrets.Failure("/commerce-secrets/STRIPE_SECRET_KEY"))
	}
	applyAccounts(ctx, pol) // the periodic re-read
	if key() != "value" {
		t.Fatal("the re-read did not pick the secret up")
	}
	if p := chargeProblems(ctx, pol); len(p) != 0 {
		t.Fatalf("problems after the secret resolved: %v", p)
	}
	if secrets.Failure("/commerce-secrets/STRIPE_SECRET_KEY") != "" {
		t.Fatal("the failure was not cleared")
	}

	secrets.Set(&flakySecrets{fails: 1 << 30, err: secrets.ErrDenied})
	applyAccounts(ctx, pol)
	applyAccounts(ctx, pol)
	p := strings.Join(chargeProblems(ctx, pol), "\n")
	if !strings.Contains(p, "/commerce-secrets/STRIPE_SECRET_KEY: permission denied") || strings.Contains(p, "value") {
		t.Fatalf("an always-failing reader must end in problems naming ref and reason: %s", p)
	}
	if secrets.Failure("/commerce-secrets/STRIPE_SECRET_KEY") != "permission denied" || key() != "" {
		t.Fatal("the failure was not recorded or the key is set")
	}
}

// A testnet seed (secrets under /cloud) names every ref under /cloud, including a secret that
// is not sealed yet; the ref that does not exist then shows up in problems.
func TestATestnetSeedDerivesEveryRefFromTheNetworksRoot(t *testing.T) {
	ctx := context.Background()
	useSecrets(t, mapSecrets{"/cloud/STRIPE_SECRET_KEY": "k"}) // the webhook secret is not there
	pol := routing.Default()
	seedProcessorAccounts(ctx, &pol, map[string]string{"STRIPE_SECRET_KEY": "x", "STRIPE_WEBHOOK_SECRET": "y"})
	var stripe routing.Account
	for _, a := range pol.Accounts {
		if a.Processor == "stripe" {
			stripe = a
		}
	}
	for _, n := range stripe.Secrets {
		if !strings.HasPrefix(n.Value, "/cloud/") {
			t.Fatalf("%s ref = %s, not under the network's root /cloud", n.Name, n.Value)
		}
	}
	if len(stripe.Secrets) != 2 {
		t.Fatalf("stripe secrets = %v", stripe.Secrets)
	}
	pol.Accounts = []routing.Account{stripe}
	stripe.Secrets = stripe.Secrets[:2]
	if p := strings.Join(chargeProblems(ctx, pol), "\n"); p != "" && strings.Contains(p, "secretKey") {
		t.Fatalf("the key resolves under /cloud: %s", p)
	}
}

// A missing webhook secret is a problem of its own kind: Stripe stays available (the money can
// be taken) and the policy stays chargeable, but credits cannot be confirmed.
func TestAMissingWebhookSecretIsASettlementProblemAndNotUnchargeable(t *testing.T) {
	ctx := context.Background()
	useSecrets(t, mapSecrets{"/cloud/STRIPE_SECRET_KEY": "k"})
	pol := routing.Default()
	for i := range pol.Processors {
		pol.Processors[i].On = pol.Processors[i].Processor == "stripe"
	}
	pol.Accounts = []routing.Account{{Processor: "stripe", ID: "live", On: true, Cards: true, Secrets: []routing.Named{
		{Name: "secretKey", Value: "/cloud/STRIPE_SECRET_KEY"}, {Name: "webhook", Value: "/cloud/STRIPE_WEBHOOK_SECRET"}}}}
	if got := chargeProblems(ctx, pol); len(got) != 0 || !canCharge(ctx, pol) {
		t.Fatalf("a missing webhook secret made the policy unchargeable: %v", got)
	}
	sp := settlementProblems(ctx, pol)
	if len(sp) != 1 || !strings.HasPrefix(sp[0], "settlement: ") || !strings.Contains(sp[0], "/cloud/STRIPE_WEBHOOK_SECRET: reader error: no such secret") {
		t.Fatalf("settlement problems = %v", sp)
	}
	secrets.Set(mapSecrets{"/cloud/STRIPE_SECRET_KEY": "k", "/cloud/STRIPE_WEBHOOK_SECRET": "w"})
	if sp := settlementProblems(ctx, pol); len(sp) != 0 {
		t.Fatalf("resolved, still reported: %v", sp)
	}
}

// A seeded charge account takes cards (without the flag the routing counts the processor
// unusable); a card-less account of an on-processor is unchargeable; and the whole seed passes
// the validation an admin write does, with an invalid MPC/Kraken account left out, not stored.
func TestASeedOfTestnetChargeAccountsTakesCardsAndValidates(t *testing.T) {
	ctx := context.Background()
	useSecrets(t, mapSecrets{
		"/cloud/STRIPE_SECRET_KEY": "k", "/cloud/STRIPE_WEBHOOK_SECRET": "w",
		"/cloud/SQUARE_ACCESS_TOKEN": "a", "/cloud/PAYPAL_CLIENT_SECRET": "c",
	})
	pol := routing.Default()
	seedFromEnvironment(ctx, &pol, []string{
		"STRIPE_SECRET_KEY=x", "STRIPE_WEBHOOK_SECRET=y", "SQUARE_ACCESS_TOKEN=a", "SQUARE_LOCATION_ID=L",
		"PAYPAL_CLIENT_SECRET=c", "PAYPAL_CLIENT_ID=i",
		"MPC_ENDPOINT=http://old.example",         // not https: an admin write refuses it
		"KRAKEN_API_KEY=k", "KRAKEN_API_SECRET=s", // unresolved: refused
	})
	var charge int
	for _, a := range pol.Accounts {
		switch a.Processor {
		case "stripe", "square", "paypal":
			charge++
			if !a.Cards {
				t.Errorf("%s %s seeded with cards false", a.Processor, a.ID)
			}
			for _, n := range a.Secrets {
				if !strings.HasPrefix(n.Value, "/cloud/") {
					t.Errorf("%s ref %s is not under /cloud", n.Name, n.Value)
				}
			}
		case "mpc", "kraken":
			t.Errorf("an invalid %s account was seeded: %+v", a.Processor, a)
		}
	}
	if charge == 0 {
		t.Fatal("no charge account seeded")
	}
	if why := validateAccounts(ctx, pol); why != "" {
		t.Fatalf("the seeded policy does not validate: %s", why)
	}
	// A charge account that takes no cards leaves its on-processor unchargeable.
	for i := range pol.Accounts {
		pol.Accounts[i].Cards = false
	}
	if p := strings.Join(chargeProblems(ctx, pol), "\n"); !strings.Contains(p, "unchargeable") || !strings.Contains(p, "takes no cards") {
		t.Fatalf("card-less accounts not flagged: %s", p)
	}
}

// The shapes live on prod (v3) and testnet (v11): admin-written policies with an empty accounts
// list, with or without seeded:true and seedLevel. The top-up must run on each and end with
// Stripe, Square and PayPal accounts that take cards, refs under the network's root, a policy
// that validates, and nothing the admin set overwritten. The fixtures hold no values.
func TestTheTopUpRunsOnTheAdminWrittenPoliciesLiveOnProdAndTestnet(t *testing.T) {
	for _, fx := range []struct {
		name, root, file string
		version          int
		seeded           bool
		level            bool
	}{
		{"prod v3, seeded flag, no level", "/commerce-secrets", "policy-prod-v3.json", 3, true, false},
		{"prod v3, seeded flag and level 2", "/commerce-secrets", "policy-prod-v3.json", 3, true, true},
		{"prod v3, no seeded flag", "/commerce-secrets", "policy-prod-v3.json", 3, false, false},
		{"testnet v11, seeded flag, no level", "/cloud", "policy-testnet-v11.json", 11, true, false},
		{"testnet v11, seeded flag and level 2", "/cloud", "policy-testnet-v11.json", 11, true, true},
		{"testnet v11, no seeded flag", "/cloud", "policy-testnet-v11.json", 11, false, false},
	} {
		for _, shape := range []string{"accounts null", "accounts empty list"} {
			t.Run(fx.name+", "+shape, func(t *testing.T) {
				ctx := heldStore(t)
				t.Setenv("STRIPE_SECRET_KEY", "x")
				t.Setenv("STRIPE_WEBHOOK_SECRET", "x")
				t.Setenv("SQUARE_ACCESS_TOKEN", "x")
				t.Setenv("SQUARE_LOCATION_ID", "LOC")
				t.Setenv("PAYPAL_CLIENT_ID", "pp")
				t.Setenv("PAYPAL_CLIENT_SECRET", "x")
				useSecrets(t, mapSecrets{
					fx.root + "/STRIPE_SECRET_KEY": "v", fx.root + "/STRIPE_WEBHOOK_SECRET": "v",
					fx.root + "/SQUARE_ACCESS_TOKEN": "v", fx.root + "/PAYPAL_CLIENT_SECRET": "v",
				})
				// The policy as the admin routing GET returned it live (testdata, refs and paths only).
				raw, err := os.ReadFile("testdata/" + fx.file)
				if err != nil {
					t.Fatal(err)
				}
				var old routing.Policy
				if err := json.Unmarshal(raw, &old); err != nil {
					t.Fatal(err)
				}
				if len(old.Accounts) != 0 {
					t.Fatal("the live fixture holds accounts")
				}
				wantFallback := append([]string(nil), old.Fallback...)
				row := policyRow(old.Normal(), "z@hanzo.ai", fx.version)
				var m map[string]json.RawMessage
				_ = json.Unmarshal([]byte(row.Rules), &m)
				if !fx.level {
					delete(m, "seedLevel")
				}
				if !fx.seeded {
					delete(m, "seeded")
				}
				if shape == "accounts null" {
					m["accounts"] = json.RawMessage("null") // the admin GET returns null, not []
				}
				b, _ := json.Marshal(m)
				row.Rules = string(b)
				if err := routingpolicy.Put(ctx, routingpolicy.Platform, row); err != nil {
					t.Fatal(err)
				}
				if err := routingpolicy.PutVersion(ctx, routingpolicy.Version{Scope: routingpolicy.Platform, N: fx.version, Policy: "{}", By: "z@hanzo.ai"}); err != nil {
					t.Fatal(err)
				}
				before, _, _ := ReadRoutingVersioned(ctx, "")
				if canCharge(ctx, before) {
					t.Fatal("the fixture can already charge; it is not the live shape")
				}

				if err := seedRuntime(ctx); err != nil {
					t.Fatal(err)
				}
				after, v, _ := ReadRoutingVersioned(ctx, "")
				if v != fx.version+1 {
					t.Fatalf("version = %d; want %d (one top-up)", v, fx.version+1)
				}
				for _, proc := range []string{"stripe", "square", "paypal"} {
					found := false
					for _, a := range after.Accounts {
						if a.Processor != proc {
							continue
						}
						found = true
						if !a.Cards {
							t.Errorf("%s %s cards = false", proc, a.ID)
						}
						for _, n := range a.Secrets {
							if !strings.HasPrefix(n.Value, fx.root+"/") {
								t.Errorf("%s %s ref %s is not under %s", proc, n.Name, n.Value, fx.root)
							}
						}
					}
					if !found {
						t.Errorf("no %s account after the top-up", proc)
					}
				}
				if why := validateAccounts(ctx, after); why != "" {
					t.Fatalf("the topped-up policy does not validate: %s", why)
				}
				if got := chargeProblems(ctx, after); len(got) != 0 {
					t.Fatalf("problems after the top-up: %v", got)
				}
				if strings.Join(after.Fallback, ",") != strings.Join(wantFallback, ",") {
					t.Fatalf("the admin's fallback was overwritten: %v -> %v", wantFallback, after.Fallback)
				}
				if err := seedRuntime(ctx); err != nil {
					t.Fatal(err)
				}
				if _, v2, _ := ReadRoutingVersioned(ctx, ""); v2 != v {
					t.Fatalf("a second boot moved the version: %d -> %d", v, v2)
				}
				// Kraken and MPC are off though the rails still list them: crypto offers only the
				// Stripe stablecoins.
				reg := processor.NewRegistry(processor.DefaultConfig())
				reg.Register(newRail(processor.Stripe, true))
				got := map[string]string{}
				for _, r := range cryptoRoutesFor(ctx, reg, after) {
					got[r.Asset] = r.Processor
				}
				if len(got) != 3 || got["usdc"] != "stripe" || got["usdp"] != "stripe" || got["usdg"] != "stripe" {
					t.Fatalf("crypto routes = %v; want usdc, usdp, usdg via stripe only", got)
				}
			})
		}
	}
}

// Secrets kept at Hanzo's org root are found by the seed, and name that root.
func TestASeedFindsSecretsAtTheOrgRoot(t *testing.T) {
	ctx := context.Background()
	useSecrets(t, mapSecrets{"/orgs/hanzo/commerce-secrets/STRIPE_SECRET_KEY": "k"})
	pol := routing.Default()
	seedProcessorAccounts(ctx, &pol, map[string]string{"STRIPE_SECRET_KEY": "x", "STRIPE_WEBHOOK_SECRET": "y"})
	for _, a := range pol.Accounts {
		for _, n := range a.Secrets {
			if !strings.HasPrefix(n.Value, "/orgs/hanzo/commerce-secrets/") {
				t.Errorf("%s ref %s is not at the org root", n.Name, n.Value)
			}
		}
	}
}

// The admin view's Kraken availability is the credential read the deposit rail makes: the
// keys sealed at the org root make it available, and keys that do not read make it not.
func TestKrakenAvailabilityFollowsTheDepositRailsCredentials(t *testing.T) {
	ctx := heldStore(t)
	installRuntime(t, ctx)
	org := routingOrg(ctx, t)
	t.Cleanup(func() { configureManagedAccounts(ctx, routing.Policy{}) })

	pol := routing.Default()
	setOn(&pol, "kraken", true)
	pol.Accounts = []routing.Account{{Processor: "kraken", ID: "exchange", On: true, SecretRef: "/orgs/hanzo/commerce-secrets"}}
	available := func() bool {
		configureManagedAccounts(ctx, pol)
		v, f := ReadRoutingView(ctx, org, "")
		if f != nil {
			t.Fatal(f.Err)
		}
		for _, s := range v.Processor {
			if s.Processor == "kraken" {
				return s.Available
			}
		}
		t.Fatal("the view lists no kraken")
		return false
	}

	useSecrets(t, mapSecrets{})
	if available() {
		t.Fatal("kraken is available with no credentials")
	}
	useSecrets(t, mapSecrets{
		"/orgs/hanzo/commerce-secrets/KRAKEN_API_KEY":    "key",
		"/orgs/hanzo/commerce-secrets/KRAKEN_API_SECRET": "c2VjcmV0", // base64, as Kraken issues it
	})
	if !available() {
		t.Fatal("kraken is not available though its credentials read at the org root")
	}
}

// A testnet whose host store keeps its keys at /orgs/hanzo/cloud is seeded with refs there,
// and the test and sandbox accounts each hold a webhook secret of their own.
func TestATestnetAtTheOrgRootSeedsRefsThereAndEachAccountItsWebhook(t *testing.T) {
	ctx := context.Background()
	useSecrets(t, mapSecrets{
		"/orgs/hanzo/cloud/STRIPE_TEST_SECRET_KEY": "k", "/orgs/hanzo/cloud/STRIPE_WEBHOOK_SECRET": "w",
		"/orgs/hanzo/cloud/SQUARE_SANDBOX_ACCESS_TOKEN": "a", "/orgs/hanzo/cloud/SQUARE_WEBHOOK_SIGNATURE_KEY": "s",
	})
	pol := routing.Default()
	seedProcessorAccounts(ctx, &pol, map[string]string{
		"STRIPE_SECRET_KEY": "sk_test_123", "STRIPE_TEST_SECRET_KEY": "sk_test_456", "STRIPE_WEBHOOK_SECRET": "y",
		"SQUARE_SANDBOX_ACCESS_TOKEN": "x", "SQUARE_WEBHOOK_SIGNATURE_KEY": "y", "SQUARE_ENVIRONMENT": "sandbox",
	})
	want := map[string]string{"stripe/test": "webhook", "square/sandbox": "webhookSignature"}
	for _, a := range pol.Accounts {
		for _, n := range a.Secrets {
			if !strings.HasPrefix(n.Value, "/orgs/hanzo/cloud/") {
				t.Errorf("%s %s %s ref %s is not at the testnet org root", a.Processor, a.ID, n.Name, n.Value)
			}
		}
		if f, ok := want[a.Processor+"/"+a.ID]; ok {
			if routing.Get(a.Secrets, f) == "" {
				t.Errorf("%s %s holds no %s", a.Processor, a.ID, f)
			}
			delete(want, a.Processor+"/"+a.ID)
		}
	}
	if len(want) != 0 {
		t.Fatalf("accounts not seeded: %v", want)
	}
	if sp := settlementProblems(ctx, pol); len(sp) != 0 {
		t.Fatalf("settlement problems on a seeded testnet: %v", sp)
	}
}

// On production a test-mode account never holds a live secret ref: it gets a webhook secret
// of its own when one is provisioned, and none otherwise.
func TestAProductionTestAccountHoldsNoLiveWebhookRef(t *testing.T) {
	ctx := context.Background()
	useSecrets(t, mapSecrets{"/orgs/hanzo/commerce-secrets/STRIPE_SECRET_KEY": "k", "/orgs/hanzo/commerce-secrets/STRIPE_WEBHOOK_SECRET": "live"})
	live := map[string]string{
		"STRIPE_SECRET_KEY": "sk_live_123", "STRIPE_WEBHOOK_SECRET": "whsec_live", "STRIPE_TEST_SECRET_KEY": "sk_test_456",
		"SQUARE_ACCESS_TOKEN": "x", "SQUARE_WEBHOOK_SIGNATURE_KEY": "sig_live", "SQUARE_SANDBOX_ACCESS_TOKEN": "y",
	}
	webhookOf := func(env map[string]string, proc, id, field string) string {
		pol := routing.Default()
		seedProcessorAccounts(ctx, &pol, env)
		for _, a := range pol.Accounts {
			if a.Processor == proc && a.ID == id {
				return routing.Get(a.Secrets, field)
			}
		}
		t.Fatalf("no %s %s account", proc, id)
		return ""
	}
	if got := webhookOf(live, "stripe", "live", "webhook"); !strings.HasSuffix(got, "/STRIPE_WEBHOOK_SECRET") {
		t.Fatalf("the live account lost its webhook: %q", got)
	}
	if got := webhookOf(live, "stripe", "test", "webhook"); got != "" {
		t.Fatalf("the production test account holds the live webhook ref %q", got)
	}
	if got := webhookOf(live, "square", "sandbox", "webhookSignature"); got != "" {
		t.Fatalf("the production sandbox account holds the live signature ref %q", got)
	}
	own := map[string]string{}
	for k, v := range live {
		own[k] = v
	}
	own["STRIPE_TEST_WEBHOOK_SECRET"], own["SQUARE_SANDBOX_WEBHOOK_SIGNATURE_KEY"] = "whsec_test", "sig_test"
	if got := webhookOf(own, "stripe", "test", "webhook"); !strings.HasSuffix(got, "/STRIPE_TEST_WEBHOOK_SECRET") {
		t.Fatalf("the test account's own webhook ref = %q", got)
	}
	if got := webhookOf(own, "square", "sandbox", "webhookSignature"); !strings.HasSuffix(got, "/SQUARE_SANDBOX_WEBHOOK_SIGNATURE_KEY") {
		t.Fatalf("the sandbox account's own signature ref = %q", got)
	}
}

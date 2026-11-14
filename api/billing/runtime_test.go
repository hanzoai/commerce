package billing

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
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

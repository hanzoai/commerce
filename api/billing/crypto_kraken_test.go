package billing

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hanzoai/commerce/billing/bucket"
	"github.com/hanzoai/commerce/billing/depositledger"
	"github.com/hanzoai/commerce/billing/depositwatch"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/cryptopaymentintent"
	"github.com/hanzoai/commerce/models/organization"
	txutil "github.com/hanzoai/commerce/models/transaction/util"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/payment/providers/kraken"
	"github.com/hanzoai/commerce/payment/routing"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/commerce/util/test/ae"
)

// TestCreateCryptoDeposit_KrakenOneFreshAddressPerPayment proves that when
// Kraken is configured as our crypto exchange processor:
//  1. cryptoProcessor(ctx) selects processor.Kraken.
//  2. Every call to CreateCryptoDeposit calls Kraken's DepositAddresses API
//     with new=true and issues a brand-new deposit address (1 address per
//     payment) — never reusing an old address across payments, so external
//     observers cannot track or correlate payments across our wallets.
func TestCreateCryptoDeposit_KrakenOneFreshAddressPerPayment(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()

	installWatcher(t, baseUSDCEnv(), true)
	krakenOn := routing.Default()
	setOn(&krakenOn, "kraken", true)
	krakenOn.Accounts = accountsOf(baseUSDCEnv())
	krakenOn.CryptoRails = []routing.CryptoRoute{{Asset: "usdc", Processor: "kraken"}}
	if _, f := WriteRouting(c, "", krakenOn, "test", ""); f != nil {
		t.Fatalf("policy: %v", f.Err)
	}

	var addrSeq int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		vals, _ := url.ParseQuery(string(body))
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/0/private/DepositMethods":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error": []string{},
				"result": []kraken.DepositMethod{
					{Method: "USDC (Base)", GenAddress: true},
				},
			})
		case "/0/private/DepositAddresses":
			if vals.Get("new") != "true" {
				t.Errorf("expected new=true on Kraken DepositAddresses, got %q", vals.Get("new"))
			}
			n := atomic.AddInt64(&addrSeq, 1)
			addr := fmt.Sprintf("0x%040x", n)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error": []string{},
				"result": []kraken.DepositAddress{
					{Address: addr, ExpireTm: "0", New: true},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	secret := base64.StdEncoding.EncodeToString([]byte("test-kraken-secret-key-for-hmac-sha512"))
	kp := kraken.NewProcessor(kraken.Config{
		APIKey:    "test-kraken-key",
		APISecret: secret,
		APIURL:    srv.URL,
	})
	processor.Register(kp)
	t.Cleanup(func() {
		processor.Register(kraken.NewProcessor(kraken.Config{}))
	})

	db := datastore.New(c)
	org := organization.New(db)
	org.Name = "acme"
	if err := org.Create(); err != nil {
		t.Fatalf("create org: %v", err)
	}

	dep1, err := CreateCryptoDeposit(context.Background(), org, "acme/alice", "base", "usdc", 2000)
	if err != nil {
		t.Fatalf("CreateCryptoDeposit 1: %v", err)
	}
	dep2, err := CreateCryptoDeposit(context.Background(), org, "acme/alice", "base", "usdc", 5000)
	if err != nil {
		t.Fatalf("CreateCryptoDeposit 2: %v", err)
	}
	dep3, err := CreateCryptoDeposit(context.Background(), org, "acme/bob", "base", "usdc", 2000)
	if err != nil {
		t.Fatalf("CreateCryptoDeposit 3: %v", err)
	}

	if dep1.DepositAddress == "" || dep2.DepositAddress == "" || dep3.DepositAddress == "" {
		t.Fatalf("missing deposit address: dep1=%+v dep2=%+v dep3=%+v", dep1, dep2, dep3)
	}
	if dep1.DepositAddress == dep2.DepositAddress {
		t.Fatalf("reused deposit address across two payments for same payer: %q", dep1.DepositAddress)
	}
	if dep2.DepositAddress == dep3.DepositAddress || dep1.DepositAddress == dep3.DepositAddress {
		t.Fatalf("reused deposit address across payers: %q / %q / %q", dep1.DepositAddress, dep2.DepositAddress, dep3.DepositAddress)
	}
	if dep1.ID == dep2.ID {
		t.Fatalf("reused ID across two separate payments: %q", dep1.ID)
	}
}

// TestKrakenRollingFiveAddressPool_CrossOrgUnifiedEVMAndPostTxRotation proves:
//  1. 5 buyers across 5 different orgs and different Unified EVM chains (base, arbitrum,
//     ethereum, optimism, polygon) receive all 5 distinct Kraken pool addresses with zero
//     cross-org or cross-chain collision.
//  2. Same-buyer page refresh with the same quote reuses their active intent without
//     consuming an extra pool slot, and Kraken's $2.50 / $2.00 method minimum is surfaced.
//  3. Once Buyer 1's deposit settles, Buyer 1's wallet is permanently retired, Kraken
//     immediately mints a brand-new 6th wallet (new=true), and Buyer 6 receives Addr #6
//     (never reusing Buyer 1's settled wallet).
func TestKrakenRollingFiveAddressPool_CrossOrgUnifiedEVMAndPostTxRotation(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()

	var mu sync.Mutex
	var seq int
	var pool []kraken.DepositAddress
	for i := 1; i <= kraken.DefaultAddressPoolSize; i++ {
		seq = i
		pool = append(pool, kraken.DepositAddress{
			Address: fmt.Sprintf("0x%040x", i),
			New:     true,
		})
	}
	var deposits []kraken.DepositRecord
	var newMintsAfterCap int64

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		vals, _ := url.ParseQuery(string(body))
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/0/private/DepositMethods":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error": []string{},
				"result": []kraken.DepositMethod{
					{Method: "USDC - Base (Unified)", GenAddress: true, Minimum: "2.50"},
					{Method: "USDC - Ethereum (Unified)", GenAddress: true, Minimum: "2.50"},
					{Method: "USDC - Arbitrum One (Unified)", GenAddress: true, Minimum: "2.50"},
					{Method: "USDC - Optimism (Unified)", GenAddress: true, Minimum: "2.50"},
					{Method: "USDC - Polygon (Unified)", GenAddress: true, Minimum: "2.00"},
				},
			})
		case "/0/private/DepositAddresses":
			mu.Lock()
			defer mu.Unlock()
			unused := 0
			for _, a := range pool {
				if a.New {
					unused++
				}
			}
			if vals.Get("new") == "true" {
				if unused >= kraken.DefaultAddressPoolSize {
					_ = json.NewEncoder(w).Encode(map[string]interface{}{
						"error":  []string{"EFunding:Too many addresses"},
						"result": []kraken.DepositAddress{},
					})
					return
				}
				seq++
				atomic.AddInt64(&newMintsAfterCap, 1)
				minted := kraken.DepositAddress{Address: fmt.Sprintf("0x%040x", seq), New: true}
				pool = append([]kraken.DepositAddress{minted}, pool...)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"error":  []string{},
					"result": []kraken.DepositAddress{minted},
				})
				return
			}
			cp := make([]kraken.DepositAddress, len(pool))
			copy(cp, pool)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error":  []string{},
				"result": cp,
			})
		case "/0/private/DepositStatus":
			mu.Lock()
			cp := make([]kraken.DepositRecord, len(deposits))
			copy(cp, deposits)
			mu.Unlock()
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error":  []string{},
				"result": cp,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	secret := base64.StdEncoding.EncodeToString([]byte("test-kraken-secret-key-for-hmac-sha512-pool-test!!"))
	useSecrets(t, mapSecrets{
		"/orgs/hanzo/commerce-secrets/KRAKEN_API_KEY":    "test-kraken-key",
		"/orgs/hanzo/commerce-secrets/KRAKEN_API_SECRET": secret,
	})
	kp := kraken.NewProcessor(kraken.Config{
		APIKey:    "test-kraken-key",
		APISecret: secret,
		APIURL:    srv.URL,
	})
	processor.Register(kp)
	t.Cleanup(func() {
		processor.Register(kraken.NewProcessor(kraken.Config{}))
	})

	krakenAcct := routing.Account{
		Processor: "kraken",
		ID:        "exchange",
		Endpoint:  srv.URL,
		SecretRef: "/orgs/hanzo/commerce-secrets",
		On:        true,
	}
	pol := routing.Default()
	setOn(&pol, "kraken", true)
	pol.Accounts = []routing.Account{{
		Processor: "kraken",
		ID:        "exchange",
		SecretRef: "/orgs/hanzo/commerce-secrets",
		On:        true,
	}}
	pol.CryptoRails = []routing.CryptoRoute{{Asset: "usdc", Processor: "kraken"}}
	if _, f := WriteRouting(c, "", pol, "test", ""); f != nil {
		t.Fatalf("policy: %v", f.Err)
	}

	svc, err := depositledger.New([]routing.Account{krakenAcct})
	if err != nil {
		t.Fatalf("depositledger.New: %v", err)
	}
	svc.Start()
	depositledger.SetDefault(svc)
	t.Cleanup(func() {
		svc.Stop()
		depositledger.SetDefault(nil)
	})

	makeOrg := func(name string) *organization.Organization {
		o := organization.New(datastore.New(c))
		o.Name = name
		o.Live = true
		if err := o.Create(); err != nil {
			t.Fatalf("create org %s: %v", name, err)
		}
		return o
	}

	orgs := []*organization.Organization{
		makeOrg("org-alpha"),
		makeOrg("org-beta"),
		makeOrg("org-gamma"),
		makeOrg("org-delta"),
		makeOrg("org-epsilon"),
	}
	chains := []string{"base", "arbitrum", "ethereum", "optimism", "polygon"}
	deps := make([]CryptoDeposit, 5)
	seen := make(map[string]string)

	for i := 0; i < 5; i++ {
		payer := fmt.Sprintf("%s/buyer-%d", orgs[i].Name, i+1)
		dep, err := CreateCryptoDeposit(context.Background(), orgs[i], payer, chains[i], "usdc", 2500)
		if err != nil {
			t.Fatalf("CreateCryptoDeposit buyer %d (%s/%s): %v", i+1, orgs[i].Name, chains[i], err)
		}
		if prev, dup := seen[dep.DepositAddress]; dup {
			t.Fatalf("cross-org/cross-chain collision: %s got %s already held by %s", payer, dep.DepositAddress, prev)
		}
		seen[dep.DepositAddress] = payer
		deps[i] = dep
	}
	if deps[0].MinimumCents != 250 {
		t.Fatalf("base minimumCents = %d, want 250 ($2.50)", deps[0].MinimumCents)
	}
	if deps[4].MinimumCents != 200 {
		t.Fatalf("polygon minimumCents = %d, want 200 ($2.00)", deps[4].MinimumCents)
	}

	// Same-buyer refresh with exact same quote reuses their active intent without burning a slot.
	refresh, err := CreateCryptoDeposit(context.Background(), orgs[0], "org-alpha/buyer-1", "base", "usdc", 2500)
	if err != nil || refresh.ID != deps[0].ID || refresh.DepositAddress != deps[0].DepositAddress {
		t.Fatalf("same-quote refresh = (%+v, %v), want reuse of %s", refresh, err, deps[0].ID)
	}

	// Buyer 6 in a 6th org while all 5 pool slots are freshly held (<15m) is refused cleanly.
	org6 := makeOrg("org-zeta")
	if _, err := CreateCryptoDeposit(context.Background(), org6, "org-zeta/buyer-6", "base", "usdc", 2500); err == nil {
		t.Fatal("expected refusal when all 5 pool addresses are actively held")
	}

	// Buyer 1's $25.00 USDC deposit lands and settles on Kraken:
	//  - Kraken flips deps[0].DepositAddress to New:false
	//  - SyncOnce credits Buyer 1, retires deps[0].DepositAddress, and immediately mints 0x...06!
	settledAddr := deps[0].DepositAddress
	mu.Lock()
	for i := range pool {
		if pool[i].Address == settledAddr {
			pool[i].New = false
		}
	}
	deposits = append(deposits, kraken.DepositRecord{
		Method: "USDC - Base (Unified)",
		Asset:  "USDC",
		RefID:  "KRAKEN-REF-1",
		TxID:   "0xsettledtx001",
		Info:   settledAddr,
		Amount: "25.00",
		Status: "Success",
		Time:   time.Now().Unix(),
	})
	mu.Unlock()

	credited, err := svc.SyncOnce(context.Background())
	if err != nil {
		t.Fatalf("SyncOnce: %v", err)
	}
	if credited != 1 {
		t.Fatalf("credited = %d, want 1", credited)
	}
	if atomic.LoadInt64(&newMintsAfterCap) < 1 {
		t.Fatal("expected post-settlement rotation to call DepositAddresses(new=true) immediately")
	}

	// Buyer 6 now gets the newly minted 6th address (0x...06), NEVER the settled 1st address!
	dep6, err := CreateCryptoDeposit(context.Background(), org6, "org-zeta/buyer-6", "base", "usdc", 3000)
	if err != nil {
		t.Fatalf("CreateCryptoDeposit buyer 6 after rotation: %v", err)
	}
	wantAddr6 := fmt.Sprintf("0x%040x", 6)
	if dep6.DepositAddress != wantAddr6 {
		t.Fatalf("buyer 6 received %s, want newly rotated %s (settled was %s)", dep6.DepositAddress, wantAddr6, settledAddr)
	}
	if dep6.DepositAddress == settledAddr {
		t.Fatalf("settled address %s was reused!", settledAddr)
	}
}

type singleTxChain struct {
	head     uint64
	transfer depositwatch.Transfer
}

func (s *singleTxChain) BlockNumber(context.Context) (uint64, error) { return s.head, nil }
func (s *singleTxChain) Decimals(context.Context) (int, error)       { return 6, nil }
func (s *singleTxChain) Symbol(context.Context) (string, error)      { return "USDC", nil }
func (s *singleTxChain) TransfersTo(_ context.Context, addrs []string, from, to uint64) ([]depositwatch.Transfer, error) {
	if s.transfer.Block < from || s.transfer.Block > to {
		return nil, nil
	}
	for _, a := range addrs {
		if a == s.transfer.To {
			return []depositwatch.Transfer{s.transfer}, nil
		}
	}
	return nil, nil
}

// TestKrakenCrossWatcherIdempotency_ChainRPCAndKrakenStatusCreditOnce proves that when
// BOTH the on-chain Base RPC watcher and Kraken's DepositStatus watcher observe the same
// deposit transaction hash, the customer is credited 1x (never 2x).
func TestKrakenCrossWatcherIdempotency_ChainRPCAndKrakenStatusCreditOnce(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()

	const depositAddr = "0x00000000000000000000000000000000000000a1"
	const txHash = "0xdeadbeefcafebabe000000000000000000000001"
	nowUnix := time.Now().Unix()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/0/private/DepositMethods":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error":  []string{},
				"result": []kraken.DepositMethod{{Method: "USDC - Base (Unified)", GenAddress: true, Minimum: "2.50"}},
			})
		case "/0/private/DepositAddresses":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error":  []string{},
				"result": []kraken.DepositAddress{{Address: depositAddr, New: true}},
			})
		case "/0/private/DepositStatus":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error": []string{},
				"result": []kraken.DepositRecord{{
					Method: "USDC - Base (Unified)",
					Asset:  "USDC",
					RefID:  "KRAKEN-DUP-TEST-1",
					TxID:   txHash,
					Info:   depositAddr,
					Amount: "50.00",
					Status: "Success",
					Time:   nowUnix,
				}},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	secret := base64.StdEncoding.EncodeToString([]byte("test-kraken-secret-key-for-cross-watcher-dedup!!"))
	useSecrets(t, mapSecrets{
		"/orgs/hanzo/commerce-secrets/KRAKEN_API_KEY":    "test-kraken-key",
		"/orgs/hanzo/commerce-secrets/KRAKEN_API_SECRET": secret,
	})
	processor.Register(kraken.NewProcessor(kraken.Config{APIKey: "test-kraken-key", APISecret: secret, APIURL: srv.URL}))
	t.Cleanup(func() { processor.Register(kraken.NewProcessor(kraken.Config{})) })

	org := organization.New(datastore.New(c))
	org.Name = "deduporg"
	org.Live = true
	if err := org.Create(); err != nil {
		t.Fatal(err)
	}
	orgNS := datastore.New(nscontext.WithNamespace(context.Background(), org.Name))
	in := cryptopaymentintent.New(orgNS)
	in.Currency = "usd"
	in.Chain = cryptopaymentintent.Base
	in.Token = "usdc"
	in.DepositAddress = depositAddr
	in.WalletID = "kraken:base:usdc:" + depositAddr
	in.CustomerRef = "deduporg/alice"
	in.Status = cryptopaymentintent.Pending
	in.ExpiresAt = time.Now().Add(30 * time.Minute)
	in.Defaults()
	if err := in.Create(); err != nil {
		t.Fatal(err)
	}

	fakeRPC := &singleTxChain{
		head: 500,
		transfer: depositwatch.Transfer{
			To:         depositAddr,
			Units:      big.NewInt(50_000_000), // $50.00 USDC (6 decimals)
			TxHash:     txHash,
			EventIndex: 0,
			Block:      450,
		},
	}
	accts := append(accountsOf(baseUSDCEnv()), routing.Account{
		Processor: "kraken",
		ID:        "exchange",
		Endpoint:  srv.URL,
		SecretRef: "/orgs/hanzo/commerce-secrets",
		On:        true,
	})
	svc, err := depositledger.New(accts, depositledger.WithReader(func(depositwatch.Asset) (depositwatch.Reader, error) {
		return fakeRPC, nil
	}))
	if err != nil {
		t.Fatalf("depositledger.New: %v", err)
	}

	// Run two passes so both st.watcher (RPC) and syncKraken process the exact same txHash.
	for pass := 1; pass <= 2; pass++ {
		if _, err := svc.SyncOnce(context.Background()); err != nil {
			t.Fatalf("SyncOnce pass %d: %v", pass, err)
		}
	}

	raw, err := txutil.GetRawByCurrency(orgNS.Context, "deduporg/alice", "iam-user", "usd", false)
	if err != nil {
		t.Fatalf("read balance: %v", err)
	}
	split := bucket.Compute(raw, "deduporg/alice", time.Now())
	if int64(split.Balance) != 5000 {
		t.Fatalf("balance = %d cents, want 5000 ($50.00 credited ONCE, not double-credited)", split.Balance)
	}
}

type failingRPCChain struct{}

func (failingRPCChain) BlockNumber(context.Context) (uint64, error) {
	return 0, fmt.Errorf("rpc eth_getLogs error -32011: request limit reached")
}
func (failingRPCChain) Decimals(context.Context) (int, error)  { return 6, nil }
func (failingRPCChain) Symbol(context.Context) (string, error) { return "USDC", nil }
func (failingRPCChain) TransfersTo(context.Context, []string, uint64, uint64) ([]depositwatch.Transfer, error) {
	return nil, fmt.Errorf("rpc eth_getLogs error -32011: request limit reached")
}

// TestSyncOnce_KrakenCreditsAndWarmsPoolEvenWhenChainRPCErrors proves that a rate-limited
// chain RPC in st.watcher never blocks syncKraken from warming the 5-address pool or
// crediting and rotating settled Kraken deposits.
func TestSyncOnce_KrakenCreditsAndWarmsPoolEvenWhenChainRPCErrors(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()

	const settledAddr = "0x00000000000000000000000000000000000000b1"
	var warmMints int64

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		vals, _ := url.ParseQuery(string(body))
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/0/private/DepositMethods":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error":  []string{},
				"result": []kraken.DepositMethod{{Method: "USDC - Base (Unified)", GenAddress: true, Minimum: "2.50"}, {Method: "USDC (SPL)", GenAddress: true, Minimum: "0.40"}},
			})
		case "/0/private/DepositAddresses":
			if vals.Get("new") == "true" {
				n := atomic.AddInt64(&warmMints, 1)
				if n > 5 {
					_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": []string{"EFunding:Too many addresses"}, "result": []kraken.DepositAddress{}})
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"error":  []string{},
					"result": []kraken.DepositAddress{{Address: fmt.Sprintf("0x%040x", 100+n), New: true}},
				})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error":  []string{},
				"result": []kraken.DepositAddress{{Address: settledAddr, New: false}},
			})
		case "/0/private/DepositStatus":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error": []string{},
				"result": []kraken.DepositRecord{{
					Method: "USDC - Base (Unified)",
					Asset:  "USDC",
					RefID:  "KRAKEN-RPC-DOWN-1",
					TxID:   "0xrpcdowntx1",
					Info:   settledAddr,
					Amount: "15.00",
					Status: "Success",
					Time:   time.Now().Unix(),
				}},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	secret := base64.StdEncoding.EncodeToString([]byte("test-kraken-secret-key-for-rpc-isolation-test!!"))
	useSecrets(t, mapSecrets{
		"/orgs/hanzo/commerce-secrets/KRAKEN_API_KEY":    "test-kraken-key",
		"/orgs/hanzo/commerce-secrets/KRAKEN_API_SECRET": secret,
	})
	processor.Register(kraken.NewProcessor(kraken.Config{APIKey: "test-kraken-key", APISecret: secret, APIURL: srv.URL}))
	t.Cleanup(func() { processor.Register(kraken.NewProcessor(kraken.Config{})) })

	org := organization.New(datastore.New(c))
	org.Name = "rpcdownorg"
	org.Live = true
	if err := org.Create(); err != nil {
		t.Fatal(err)
	}
	orgNS := datastore.New(nscontext.WithNamespace(context.Background(), org.Name))
	in := cryptopaymentintent.New(orgNS)
	in.Currency = "usd"
	in.Chain = cryptopaymentintent.Base
	in.Token = "usdc"
	in.DepositAddress = settledAddr
	in.WalletID = "kraken:base:usdc:" + settledAddr
	in.CustomerRef = "rpcdownorg/bob"
	in.Status = cryptopaymentintent.Pending
	in.ExpiresAt = time.Now().Add(30 * time.Minute)
	in.Defaults()
	if err := in.Create(); err != nil {
		t.Fatal(err)
	}

	accts := append(accountsOf(baseUSDCEnv()), routing.Account{
		Processor: "kraken",
		ID:        "exchange",
		Endpoint:  srv.URL,
		SecretRef: "/orgs/hanzo/commerce-secrets",
		On:        true,
	})
	svc, err := depositledger.New(accts, depositledger.WithReader(func(depositwatch.Asset) (depositwatch.Reader, error) {
		return failingRPCChain{}, nil
	}))
	if err != nil {
		t.Fatalf("depositledger.New: %v", err)
	}

	credited, syncErr := svc.SyncOnce(context.Background())
	if syncErr == nil {
		t.Fatal("expected RPC error to still be reported from SyncOnce")
	}
	if credited != 1 {
		t.Fatalf("credited = %d, want 1 even while RPC errored", credited)
	}
	if atomic.LoadInt64(&warmMints) == 0 {
		t.Fatal("expected Kraken pool warming to execute even while RPC errored")
	}

	raw, err := txutil.GetRawByCurrency(orgNS.Context, "rpcdownorg/bob", "iam-user", "usd", false)
	if err != nil {
		t.Fatalf("read balance: %v", err)
	}
	split := bucket.Compute(raw, "rpcdownorg/bob", time.Now())
	if int64(split.Balance) != 1500 {
		t.Fatalf("balance = %d cents, want 1500", split.Balance)
	}
}


package mpc

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/luxfi/zap"

	"github.com/hanzoai/commerce/util/zapwire"
)

// Everything in this file runs over EVERY transport, and that is the whole
// design of it. Two wires behind one interface are two chances to be right and
// two chances to drift; the only durable defence is that the behaviour is
// asserted once and replayed against each wire, so a rule that holds on HTTP
// and not on ZAP fails the suite rather than a customer.
//
// The rows differ only in how bytes travel. "zap-uds" and "zap-tcp" are the
// same client and the same code path — the address alone decides the medium —
// so having both also pins that property down.
var transports = []struct {
	name  string
	start func(t *testing.T, s *signer) *MPCProcessor
}{
	{"http", startHTTPSigner},
	{"zap-tcp", func(t *testing.T, s *signer) *MPCProcessor { return startZAPSigner(t, s, freeTCPAddr(t)) }},
	{"zap-uds", func(t *testing.T, s *signer) *MPCProcessor { return startZAPSigner(t, s, tempSocket(t)) }},
}

// --- the fake signer: ONE behaviour, served over every wire ---

// signerReply is what the fake mpcd answers. A non-empty fail means it refuses.
type signerReply struct {
	body map[string]string
	fail string
}

// signer is the fake custody fleet. It records what it was actually asked so a
// test can assert on the REQUEST as well as the answer — that is how "the rail
// sent a wallet id" and "the rail never reuses one" become checkable rather
// than assumed.
type signer struct {
	mu      sync.Mutex
	reply   func(orgID, walletID string) signerReply
	orgs    []string
	wallets []string
}

func (s *signer) serve(orgID, walletID string) signerReply {
	s.mu.Lock()
	s.orgs = append(s.orgs, orgID)
	s.wallets = append(s.wallets, walletID)
	reply := s.reply
	s.mu.Unlock()
	return reply(orgID, walletID)
}

func (s *signer) seen() (orgs, wallets []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.orgs...), append([]string(nil), s.wallets...)
}

// addrs is what a keygen reply carries. Named fields rather than positional
// strings because a signer that mints only an EVM address should SAY that, not
// spell it as three empty parameters — and because the next curve added here
// must not silently shift what an existing call site means.
type addrs struct{ evm, btc, sol, ton string }

// mints builds a signer that answers with the given addresses, echoing back the
// wallet id it served so a caller's handle can be checked against it. An empty
// address is simply absent from the reply, which is the live shape: a fleet
// that derives no Ed25519 key sends no sol_address at all.
func mints(a addrs) *signer {
	return &signer{reply: func(_, walletID string) signerReply {
		body := map[string]string{"wallet_id": walletID, "result_type": "keygen"}
		for k, v := range map[string]string{
			"evm_address": a.evm, "btc_address": a.btc,
			"sol_address": a.sol, "ton_address": a.ton,
		} {
			if v != "" {
				body[k] = v
			}
		}
		return signerReply{body: body}
	}}
}

// poisonAddress is an address a refusing signer also hands back. Nothing may
// ever spend to it: it exists so that "the signer said no" and "the signer gave
// us somewhere to send money" can be true at the same time, which is the shape
// that turns a missing error check into a credited deposit at an address no
// keygen ever completed for.
const poisonAddress = "0xdeaddeaddeaddeaddeaddeaddeaddeaddeaddead"

// refuses builds a signer that reports a failure the way its wire does: an
// error field in the body on HTTP, a non-zero status on ZAP. Both must land on
// the same refusal at the rail.
//
// On HTTP the refusal ALSO carries an address, because mpcd answers 200 with a
// body either way and nothing stops a failed keygen from arriving beside a
// leftover field. Reading the error is therefore what makes the rail safe —
// falling through to the chain switch would find that address and hand it out.
// ZAP cannot express the same shape (a non-zero status makes the payload the
// error text), which is exactly why the check belongs above the transport.
func refuses(msg string) *signer {
	return &signer{reply: func(_, _ string) signerReply {
		return signerReply{
			fail: msg,
			body: map[string]string{
				"evm_address": poisonAddress,
				"btc_address": poisonAddress,
				"sol_address": poisonAddress,
			},
		}
	}}
}

// startHTTPSigner serves the fake over mpcd's REST contract, including its
// server-side minting of an absent wallet id.
func startHTTPSigner(t *testing.T, s *signer) *MPCProcessor {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			return
		}
		if r.URL.Path != "/keygen" {
			t.Errorf("keygen posted to %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var req zapKeygenReq
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.OrgID == "" {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "org_id is required"})
			return
		}
		// mpcd mints the id when the caller omits it, which is exactly why the
		// HTTP wire does not send one.
		walletID := req.WalletID
		if walletID == "" {
			walletID = "node-" + randomHex(8)
		}
		reply := s.serve(req.OrgID, walletID)
		body := map[string]string{}
		for k, v := range reply.body {
			body[k] = v
		}
		if reply.fail != "" {
			// 200 with an error field is mpcd's own failure shape, and the
			// body it comes with is not guaranteed to be empty.
			body["error"] = reply.fail
			body["error_code"] = "KEYGEN_FAILED"
		}
		_ = json.NewEncoder(w).Encode(body)
	}))
	t.Cleanup(srv.Close)
	return NewProcessor(Config{KMSEndpoint: srv.URL, MPCEndpoint: srv.URL})
}

// startZAPSigner serves the same fake over a ZAP node at addr — a host:port or
// a socket path, whichever the caller chose.
//
// It enforces mpcd's ZAP-side rule that a keygen carry BOTH org_id and wallet
// id, so a client that quietly stopped sending one fails here rather than in
// production.
func startZAPSigner(t *testing.T, s *signer, addr string) *MPCProcessor {
	t.Helper()
	serveZAP(t, s, addr)

	// The HTTP endpoint is set but unreachable on purpose: if any part of the
	// keygen or health path were still using it, these tests would hang or
	// fail rather than silently pass over the wrong wire.
	return NewProcessor(Config{
		KMSEndpoint: "http://kms.invalid",
		MPCEndpoint: "http://mpc.invalid",
		ZAPAddress:  addr,
	})
}

// serveZAP starts a fake signer node at addr and returns a stop function. It is
// stopped at test end regardless, so a caller that stops it early can do so
// without bookkeeping.
func serveZAP(t *testing.T, s *signer, addr string) (stop func()) {
	t.Helper()
	node := zap.NewNode(zap.NodeConfig{
		NodeID:      "fake-mpcd",
		ServiceType: "_mpc._tcp",
		Address:     addr,
		NoDiscovery: true,
	})
	node.Handle(opMPCKeygen, func(_ context.Context, _ string, msg *zap.Message) (*zap.Message, error) {
		var req zapKeygenReq
		if err := json.Unmarshal(msg.Root().Bytes(zapwire.FieldPayload), &req); err != nil {
			return zapwire.Error("bad request: " + err.Error()), nil
		}
		if req.OrgID == "" || req.WalletID == "" {
			return zapwire.Error("org_id and wallet_id required"), nil
		}
		reply := s.serve(req.OrgID, req.WalletID)
		if reply.fail != "" {
			return zapwire.Error(reply.fail), nil
		}
		body, err := json.Marshal(reply.body)
		if err != nil {
			return zapwire.Error(err.Error()), nil
		}
		return zapwire.OK(body), nil
	})
	node.Handle(opMPCStatus, func(context.Context, string, *zap.Message) (*zap.Message, error) {
		return zapwire.OK([]byte(`{"status":"ok"}`)), nil
	})
	if err := node.Start(); err != nil {
		t.Fatalf("start fake ZAP signer on %s: %v", addr, err)
	}
	var once sync.Once
	stop = func() { once.Do(node.Stop) }
	t.Cleanup(stop)
	return stop
}

func freeTCPAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	addr := l.Addr().String()
	_ = l.Close()
	return addr
}

// tempSocket returns a short socket path. Short because a unix address is
// capped near 104 bytes and a nested temp directory can exceed it.
func tempSocket(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "mpcz")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return filepath.Join(dir, "s")
}

// --- the behaviour, asserted once per wire ---

// An address without its custody handle is half a record, and the half we kept
// was the one that cannot spend.
//
// Keygen returns both the wallet id and the per-curve addresses. This code
// parsed the wallet id and then discarded it, because GenerateAddress returned
// a bare string — so commerce minted custody addresses it held no handle to,
// and a credited deposit could never be swept without reconciling against the
// MPC node's own wallet records. The return type is a Wallet now, which makes
// dropping the id something you would have to do on purpose — on either wire.
func TestGenerateAddress_KeepsTheWalletHandle(t *testing.T) {
	const (
		wantEVM = "0x1111111111111111111111111111111111111111"
		wantBTC = "bc1qexampleexampleexampleexampleexampleex"
		wantSOL = "So11111111111111111111111111111111111111112"
		wantTON = "UQB6b9lZVanb-8w_sUn4NZ8clDs5dw9QghJxYeT87GTYRHye"
	)

	for _, tr := range transports {
		t.Run(tr.name, func(t *testing.T) {
			// Every chain branch must carry the handle — the bug was one shared
			// cause, so covering only "ethereum" would leave two ways to
			// reintroduce it.
			for _, tc := range []struct{ chain, wantAddr string }{
				{"ethereum", wantEVM},
				{"base", wantEVM}, // EVM chains share the secp256k1 address
				{"bitcoin", wantBTC},
				{"solana", wantSOL},
				{"ton", wantTON},
			} {
				t.Run(tc.chain, func(t *testing.T) {
					s := mints(addrs{evm: wantEVM, btc: wantBTC, sol: wantSOL, ton: wantTON})
					mp := tr.start(t, s)

					got, err := mp.GenerateAddress(context.Background(), "hanzo/z@hanzo.ai", tc.chain)
					if err != nil {
						t.Fatalf("GenerateAddress(%s): %v", tc.chain, err)
					}
					if got.Address != tc.wantAddr {
						t.Errorf("address = %q, want %q", got.Address, tc.wantAddr)
					}
					_, wallets := s.seen()
					if len(wallets) != 1 {
						t.Fatalf("signer saw %d keygens, want 1", len(wallets))
					}
					if got.ID == "" || got.ID != wallets[0] {
						t.Errorf("wallet id = %q, want %q — an address whose wallet we cannot name is one we cannot sweep", got.ID, wallets[0])
					}
				})
			}
		})
	}
}

// A keygen that returns no address for the requested chain must fail rather
// than hand back an empty destination. Money sent to "" is money sent nowhere,
// and an empty string is exactly what a zero-value struct would return if the
// error path were ever dropped.
func TestGenerateAddress_RefusesAnEmptyDestination(t *testing.T) {
	for _, tr := range transports {
		t.Run(tr.name, func(t *testing.T) {
			// A node that knows only secp256k1 answers with no sol_address —
			// which is the live shape while no Ed25519 ciphersuite exists.
			mp := tr.start(t, mints(addrs{evm: "0x2222222222222222222222222222222222222222"}))

			got, err := mp.GenerateAddress(context.Background(), "hanzo", "solana")
			if err == nil {
				t.Fatalf("solana keygen with no sol_address returned %+v, want an error", got)
			}
			if got.Address != "" {
				t.Errorf("failed keygen still produced address %q", got.Address)
			}
			if got.ID != "" {
				t.Errorf("failed keygen still produced wallet id %q", got.ID)
			}
		})
	}
}

// Every chain that is not bitcoin or solana takes the secp256k1 EVM address.
// That fall-through is deliberate and load-bearing: the brand chains and the
// L2s are all EVM, so one derived address serves them, and the deposit rail's
// menu is built on the assumption that it does.
//
// It is NOT a licence to mint for anything a caller names, and nothing here
// makes it one — the rail decides which chains reach custody at all (XRPL is
// pooled and never asked; solana is withheld because the fleet derives no key
// for it). This pins the mapping so a change to it has to be a decision.
func TestGenerateAddress_EVMChainsShareTheSecpAddress(t *testing.T) {
	const evm = "0x3333333333333333333333333333333333333333"

	for _, tr := range transports {
		t.Run(tr.name, func(t *testing.T) {
			for _, chain := range []string{"ethereum", "polygon", "arbitrum", "optimism", "base", "lux", "bsc"} {
				t.Run(chain, func(t *testing.T) {
					mp := tr.start(t, mints(addrs{evm: evm, btc: "bc1qbtc"}))
					got, err := mp.GenerateAddress(context.Background(), "hanzo", chain)
					if err != nil {
						t.Fatalf("GenerateAddress(%s): %v", chain, err)
					}
					if got.Address != evm {
						t.Errorf("%s address = %q, want the EVM address %q", chain, got.Address, evm)
					}
				})
			}
		})
	}
}

// A signer that refuses must produce a refusal, not a wallet — however its wire
// happens to report the failure (an error field in the body on HTTP, a non-zero
// status on ZAP), and even when it hands back an address alongside the refusal.
//
// The address in that reply is the whole point: a rail that skipped the error
// and went straight to the chain switch would find one and return it, and the
// deposit credited to it would sit at a key no completed keygen ever produced.
func TestGenerateAddress_SignerRefusalIsARefusal(t *testing.T) {
	for _, tr := range transports {
		t.Run(tr.name, func(t *testing.T) {
			for _, chain := range []string{"ethereum", "bitcoin", "solana"} {
				t.Run(chain, func(t *testing.T) {
					mp := tr.start(t, refuses("peers not ready"))

					got, err := mp.GenerateAddress(context.Background(), "hanzo", chain)
					if err == nil {
						t.Fatalf("refused keygen returned %+v, want an error", got)
					}
					if got.Address == poisonAddress {
						t.Errorf("refused keygen handed out the address it was refused with (%q)", got.Address)
					}
					if got.Address != "" || got.ID != "" {
						t.Errorf("refused keygen still produced %+v", got)
					}
				})
			}
		})
	}
}

// The org is everything before the first slash of the payer key, on every wire.
// It scopes the wallet on the MPC side, so carrying "hanzo/z@hanzo.ai" through
// as-is would file a customer's wallet under an org that does not exist.
func TestGenerateAddress_ScopesToTheOrg(t *testing.T) {
	for _, tr := range transports {
		t.Run(tr.name, func(t *testing.T) {
			s := mints(addrs{evm: "0x4444444444444444444444444444444444444444"})
			mp := tr.start(t, s)

			if _, err := mp.GenerateAddress(context.Background(), "hanzo/z@hanzo.ai", "ethereum"); err != nil {
				t.Fatalf("GenerateAddress: %v", err)
			}
			orgs, _ := s.seen()
			if len(orgs) != 1 || orgs[0] != "hanzo" {
				t.Errorf("signer was asked for org %v, want [hanzo]", orgs)
			}
		})
	}
}

// Two keygens must never share a wallet id.
//
// A replayed id RE-KEYS an existing wallet on mpcd, silently moving an address
// that funds may already be in flight to. On HTTP the node mints and the
// property is the node's; on ZAP the node refuses to mint and the property is
// ours, from crypto/rand. Asserting it on both is what stops someone deriving
// the id from the customer — note the identical payer on both calls, which is
// precisely the input a "deterministic id" mistake would key on.
func TestGenerateAddress_NeverReusesAWalletID(t *testing.T) {
	for _, tr := range transports {
		t.Run(tr.name, func(t *testing.T) {
			s := mints(addrs{evm: "0x5555555555555555555555555555555555555555"})
			mp := tr.start(t, s)

			const samePayer = "hanzo/z@hanzo.ai"
			first, err := mp.GenerateAddress(context.Background(), samePayer, "ethereum")
			if err != nil {
				t.Fatalf("first keygen: %v", err)
			}
			second, err := mp.GenerateAddress(context.Background(), samePayer, "ethereum")
			if err != nil {
				t.Fatalf("second keygen: %v", err)
			}

			if first.ID == second.ID {
				t.Errorf("both keygens for %q got wallet id %q — a replayed id re-keys a live wallet", samePayer, first.ID)
			}
			_, wallets := s.seen()
			if len(wallets) != 2 || wallets[0] == wallets[1] {
				t.Errorf("signer was asked to key %v, want two distinct wallets", wallets)
			}
			for _, w := range wallets {
				if w == "" {
					t.Error("signer was asked to key an empty wallet id")
				}
			}
		})
	}
}

// Availability must be answered over the wire the rail actually mints on. A
// rail that health-checked HTTP while keying over ZAP would report a signer it
// does not use — green while the wire that matters is down.
func TestIsAvailable_ProbesTheWireInUse(t *testing.T) {
	for _, tr := range transports {
		t.Run(tr.name, func(t *testing.T) {
			mp := tr.start(t, mints(addrs{evm: "0x6666666666666666666666666666666666666666"}))
			if !mp.IsAvailable(context.Background()) {
				t.Error("IsAvailable = false against a live signer")
			}
		})
	}
}

// A signer that goes away and comes back must be reachable again without
// restarting commerce. mpcd redeploys and pods reschedule; a rail that cached a
// dead connection forever would stop minting until it was itself restarted.
//
// The first call after the signer returns is allowed to fail, and that is a
// decision rather than a tolerance: a keygen is never silently retried. A
// transport error cannot distinguish "the request never landed" from "the reply
// was lost", and retrying the second case runs a SECOND ceremony and strands
// the wallet from the first. The rail reports the failure, the deposit endpoint
// renders its 503, and the customer's retry — a new request, deliberately made
// — lands on the reconnected wire.
func TestZAPTransport_RecoversAfterTheSignerRestarts(t *testing.T) {
	addr := tempSocket(t)
	s := mints(addrs{evm: "0x7777777777777777777777777777777777777777"})

	stopFirst := serveZAP(t, s, addr)
	mp := NewProcessor(Config{
		KMSEndpoint: "http://kms.invalid",
		MPCEndpoint: "http://mpc.invalid",
		ZAPAddress:  addr,
	})

	if _, err := mp.GenerateAddress(context.Background(), "hanzo", "ethereum"); err != nil {
		t.Fatalf("keygen against the first signer: %v", err)
	}

	stopFirst()
	serveZAP(t, s, addr) // a fresh signer at the same address

	// Poll rather than count attempts. How many calls it takes depends on
	// whether the client has already noticed the old peer drop, which is a race
	// with no interesting answer; that it recovers WITHOUT restarting commerce
	// is the property, and a deadline states it without encoding a magic
	// number. Each attempt is bounded so a call that lands on a half-closed
	// connection cannot park on the keygen ceiling.
	deadline := time.Now().Add(10 * time.Second)
	var err error
	for time.Now().Before(deadline) {
		attempt, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_, err = mp.GenerateAddress(attempt, "hanzo", "ethereum")
		cancel()
		if err == nil {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("rail never recovered after the signer restarted: %v", err)
}

// --- a chain is paid at ITS OWN address, never at a default ---

// Every chain this signer OFFERS must resolve to the address of its own curve.
//
// This is the one property that cannot be checked by looking at a returned
// address, because being wrong here does not produce garbage: it produces a
// perfectly well-formed address on a DIFFERENT network. GenerateAddress used to
// switch on chain with `default: resp.EVMAddress`, so any chain nobody had
// mapped was silently paid at the secp256k1 address — a buyer sending jetton
// USDT to an 0x string loses it, and nothing anywhere reports an error.
//
// It is driven off SupportedChains rather than a list written here, so a chain
// added to the table without an address mapping fails THIS test instead of
// shipping. That is the drift the one-table design exists to make impossible.
func TestGenerateAddress_EveryOfferedChainUsesItsOwnCurve(t *testing.T) {
	const (
		evm = "0x1111111111111111111111111111111111111111"
		btc = "bc1qexampleexampleexampleexampleexampleex"
		sol = "So11111111111111111111111111111111111111112"
		ton = "UQB6b9lZVanb-8w_sUn4NZ8clDs5dw9QghJxYeT87GTYRHye"
	)
	// What each chain must NOT be paid at. Only the non-EVM chains can express
	// the fall-through, and they are exactly the ones that lose money to it.
	want := map[string]string{"bitcoin": btc, "solana": sol, "ton": ton}

	mp := startHTTPSigner(t, mints(addrs{evm: evm, btc: btc, sol: sol, ton: ton}))
	for _, chain := range mp.SupportedChains() {
		t.Run(chain, func(t *testing.T) {
			got, err := mp.GenerateAddress(context.Background(), "hanzo", chain)
			if err != nil {
				t.Fatalf("GenerateAddress(%s): %v — an offered chain must be mintable", chain, err)
			}
			expect, ok := want[chain]
			if !ok {
				expect = evm // the EVM chains legitimately share one address
			}
			if got.Address != expect {
				t.Fatalf("%s paid at %q, want %q — a well-formed address on the wrong network is money gone",
					chain, got.Address, expect)
			}
		})
	}
}

// A chain the table does not declare is REFUSED. The old default answered every
// unknown chain with the EVM address, so "unsupported" and "here is somewhere to
// send money" were the same reply.
func TestGenerateAddress_RefusesAnUndeclaredChain(t *testing.T) {
	mp := startHTTPSigner(t, mints(addrs{evm: "0x9999999999999999999999999999999999999999"}))

	for _, chain := range []string{"xrpl", "cardano", ""} {
		t.Run("chain="+chain, func(t *testing.T) {
			got, err := mp.GenerateAddress(context.Background(), "hanzo", chain)
			if err == nil {
				t.Fatalf("GenerateAddress(%q) returned %q with no error — an undeclared chain was handed a destination", chain, got.Address)
			}
			if got.Address != "" {
				t.Errorf("refusal still carried address %q", got.Address)
			}
		})
	}
}

// XRPL must never appear among the mintable chains. Its deposits are POOLED —
// one configured custody account plus a per-payer destination tag — so a minted
// XRPL address would both strand a non-refundable base reserve and, since this
// table has no XRPL entry, be refused anyway. Offering it here would route the
// pooled chain down the per-payer door.
func TestSupportedChains_ExcludesPooledXRPL(t *testing.T) {
	mp := startHTTPSigner(t, mints(addrs{evm: "0x8888888888888888888888888888888888888888"}))
	for _, c := range mp.SupportedChains() {
		if c == "xrpl" || c == "xrp" {
			t.Fatalf("SupportedChains offers %q — XRPL is pooled and its address is configured, not minted", c)
		}
	}
}

// Solana and TON are OFFERED. Both were absent for a real reason that has since
// been fixed (the fleet ran no Ed25519 ceremony), and the comment explaining
// their absence outlived the fix by months — solana was being WATCHED while no
// buyer could ever be given an address for it. This pins the corrected state.
func TestSupportedChains_IncludesTheEd25519Chains(t *testing.T) {
	mp := startHTTPSigner(t, mints(addrs{evm: "0x7777777777777777777777777777777777777777"}))
	offered := map[string]bool{}
	for _, c := range mp.SupportedChains() {
		offered[c] = true
	}
	for _, c := range []string{"solana", "ton"} {
		if !offered[c] {
			t.Errorf("%q is not offered — the fleet derives an Ed25519 key, so an address can be minted there", c)
		}
	}
}

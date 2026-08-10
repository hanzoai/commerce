package solana

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hanzoai/commerce/billing/custody"
	"github.com/hanzoai/commerce/billing/solanarpc"
)

// --- The external vector --------------------------------------------------
//
// A real Solana transaction, built and signed by somebody else's wallet and
// finalised in mainnet slot 438,463,525. The whole cluster verified it.
//
// It anchors two things a fixture cannot. The signature in it verifies against
// the MESSAGE bytes with no hashing applied, which is the fact mpcd's FROST
// path depends on and the one a careless caller breaks by pre-hashing. And the
// message round-trips through our builder byte for byte, which is the only way
// to know our account ordering, header counts, compact-u16 lengths and
// instruction layout agree with the runtime's.
const vectorRawB64 = "AdrvHkc9+gcmw3+0Q2Pc10PetknWcZcDdhtFoIH4xf9zbZNfuX8Szo/ioD+jAajG9v8zanBwm8fi" +
	"cwmPARZbrwgBAAIDURnYWsDapxWX7ft3iKLDlVn0tDXO6adoPTCuK3emkYcDBkZv5SEXMv/srbpy" +
	"w5vnvIzlu8X3EmssQ5s6QAAAAPPz2jXlN+4Lbn9bjaX7fym5GffQDDs3T49rcR5N56eWNHmOQLOg" +
	"tLxpwpiNoVaWp2W0hGyS7SUWOM6oIBwSDKsDAQAFAi4BAAABAAkDL87QDQAAAAACACBTdONAxw7i" +
	"ESd7J0OL9cN8lWHRA+LOv8db6kmjNMlluQ=="

func vector(t *testing.T) (sig, msg []byte) {
	t.Helper()
	raw, err := base64.StdEncoding.DecodeString(vectorRawB64)
	if err != nil {
		t.Fatalf("vector is not base64: %v", err)
	}
	if raw[0] != 1 {
		t.Fatalf("vector carries %d signatures, expected 1", raw[0])
	}
	return raw[1:65], raw[65:]
}

// TestSignatureCoversTheMessageUnhashed is the fact this package is built on.
//
// Ed25519 hashes internally. A caller that handed the signer a digest would be
// signing a digest of a digest, producing 64 perfectly well-formed bytes that no
// verifier accepts — and the failure would only show up as a rejected
// transaction, with the fee already spent.
func TestSignatureCoversTheMessageUnhashed(t *testing.T) {
	sig, msg := vector(t)
	// The first account key is the fee payer and the only signer.
	signer := msg[3+1 : 3+1+32]
	if !ed25519.Verify(ed25519.PublicKey(signer), msg, sig) {
		t.Fatal("a finalised mainnet signature does not verify over the raw message bytes")
	}
	// And it must NOT verify over a hash of the message, which is the mistake
	// this test exists to keep out.
	h := sha256Of(msg)
	if ed25519.Verify(ed25519.PublicKey(signer), h, sig) {
		t.Fatal("the signature also verifies over a hash; the vector proves nothing")
	}
}

// TestMessageRoundTripsAMinedTransaction takes a real message apart and puts it
// back together with our builder, requiring the bytes to be identical.
//
// Account ordering, the three header counts, compact-u16 lengths and the
// instruction layout are all load-bearing and all invisible until the runtime
// rejects something. This is the cheapest place to find out they are right.
func TestMessageRoundTripsAMinedTransaction(t *testing.T) {
	_, msg := vector(t)
	p := parse(t, msg)

	var hash solanarpc.PublicKey
	copy(hash[:], p.blockhash)
	m := newMessage(hash)
	for i, k := range p.keys {
		var key solanarpc.PublicKey
		copy(key[:], k)
		if got := m.add(key, p.signer(i), p.writable(i)); got != uint8(i) {
			t.Fatalf("account %d landed at index %d", i, got)
		}
	}
	for _, in := range p.ins {
		m.instruct(in.program, in.accounts, in.data)
	}

	if got := m.bytes(); string(got) != string(msg) {
		t.Fatalf("rebuilt message differs from the one on chain\n got %x\nwant %x", got, msg)
	}
	t.Logf("round-tripped a %d-byte mainnet message: %d accounts, %d instructions", len(msg), len(p.keys), len(p.ins))
}

// --- a test-only reader for the vector ---

type parsed struct {
	signers, roSigners, roOthers uint8
	keys                         [][]byte
	blockhash                    []byte
	ins                          []instruction
}

func (p parsed) signer(i int) bool { return i < int(p.signers) }
func (p parsed) writable(i int) bool {
	if i < int(p.signers) {
		return i < int(p.signers)-int(p.roSigners)
	}
	return i < len(p.keys)-int(p.roOthers)
}

func parse(t *testing.T, msg []byte) parsed {
	t.Helper()
	var p parsed
	p.signers, p.roSigners, p.roOthers = msg[0], msg[1], msg[2]
	i := 3
	n := int(msg[i])
	i++
	for k := 0; k < n; k++ {
		p.keys = append(p.keys, msg[i:i+32])
		i += 32
	}
	p.blockhash = msg[i : i+32]
	i += 32
	ni := int(msg[i])
	i++
	for j := 0; j < ni; j++ {
		var in instruction
		in.program = msg[i]
		i++
		na := int(msg[i])
		i++
		in.accounts = append([]uint8{}, msg[i:i+na]...)
		i += na
		dl := int(msg[i])
		i++
		in.data = append([]byte{}, msg[i:i+dl]...)
		i += dl
		p.ins = append(p.ins, in)
	}
	if i != len(msg) {
		t.Fatalf("parsed %d of %d bytes; the vector or the parser is wrong", i, len(msg))
	}
	return p
}

func sha256Of(b []byte) []byte {
	h := sha256.Sum256(b)
	return h[:]
}

// TestShortvec covers Solana's compact-u16 at the boundaries where its length
// changes, because a wrong length byte shifts every field after it.
func TestShortvec(t *testing.T) {
	for n, want := range map[int]string{
		// Seven bits per byte, low group first, high bit set on every group but
		// the last. 16383 is the largest two-byte value and 65535 the largest
		// a compact-u16 can carry at all.
		0: "00", 1: "01", 127: "7f", 128: "8001", 129: "8101", 255: "ff01",
		16383: "ff7f", 16384: "808001", 65535: "ffff03",
	} {
		if got := hexOf(shortvec(nil, n)); got != want {
			t.Errorf("shortvec(%d) = %s, want %s", n, got, want)
		}
	}
}

// TestAddMergesAndWidens. The same key is often both the fee payer and a
// transfer's authority; listing it twice produces a message the runtime
// rejects, and merging it read-only when it must be writable produces one the
// runtime refuses to execute.
func TestAddMergesAndWidens(t *testing.T) {
	m := newMessage(solanarpc.PublicKey{})
	k := solanarpc.MustPublicKey("6Tat6ydCwuNEoqasJ1feCHeTypFWq4eQUB391PE1JLxW")
	a := m.add(k, false, false)
	b := m.add(k, true, true)
	if a != b {
		t.Fatalf("the same key landed at two indices, %d and %d", a, b)
	}
	if len(m.keys) != 1 {
		t.Fatalf("the key was listed %d times", len(m.keys))
	}
	if !m.sign[0] || !m.write[0] {
		t.Fatal("privileges narrowed instead of widening")
	}
}

// --- Draft against a fake cluster ----------------------------------------

type cluster struct {
	t         *testing.T
	lamports  uint64
	token     string // base-unit amount as a decimal string
	ataExists bool
	fee       uint64
	decimals  byte
	sent      string
	simulated string
	simErr    string
}

func (c *cluster) chain(t *testing.T) *Chain {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     int64  `json:"id"`
			Method string `json:"method"`
			Params []any  `json:"params"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		var result any
		switch req.Method {
		case "getLatestBlockhash":
			result = map[string]any{"value": map[string]any{
				"blockhash": "4Xqhsjx8FqqfqwEBjzS8efxbW2u5gYsSQ2J9qZZL3Tea", "lastValidBlockHeight": 1000,
			}}
		case "getBalance":
			result = map[string]any{"value": c.lamports}
		case "getTokenAccountBalance":
			result = map[string]any{"value": map[string]any{"amount": c.token}}
		case "getAccountInfo":
			// The mint is the only account we answer data for; anything else is
			// the destination token-account existence check.
			key, _ := req.Params[0].(string)
			if key == mintAddr {
				result = map[string]any{"value": map[string]any{
					"owner": solanarpc.TokenProgramID.String(),
					"data":  []any{base64.StdEncoding.EncodeToString(mintData(c.decimals)), "base64"},
				}}
			} else if c.ataExists {
				result = map[string]any{"value": map[string]any{"owner": solanarpc.TokenProgramID.String(), "data": []any{"", "base64"}}}
			} else {
				result = map[string]any{"value": nil}
			}
		case "getFeeForMessage":
			result = map[string]any{"value": c.fee}
		case "simulateTransaction":
			c.simulated, _ = req.Params[0].(string)
			v := map[string]any{"err": nil, "logs": []string{}}
			if c.simErr != "" {
				v["err"] = c.simErr
			}
			result = map[string]any{"value": v}
		case "sendTransaction":
			c.sent, _ = req.Params[0].(string)
			result = "5j7s...signature"
		default:
			c.t.Errorf("unexpected RPC %s — a sweep should not need it", req.Method)
		}
		json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
	}))
	t.Cleanup(srv.Close)
	return New(srv.URL)
}

// mintData builds an 82-byte SPL mint account with the given decimals.
func mintData(decimals byte) []byte {
	d := make([]byte, 82)
	d[44] = decimals
	d[45] = 1 // is_initialized
	return d
}

const mintAddr = "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v" // USDC

func keyed(t *testing.T) (ed25519.PrivateKey, string) {
	t.Helper()
	seed := make([]byte, ed25519.SeedSize)
	for i := range seed {
		seed[i] = byte(i + 1)
	}
	priv := ed25519.NewKeyFromSeed(seed)
	var pk solanarpc.PublicKey
	copy(pk[:], priv.Public().(ed25519.PublicKey))
	return priv, pk.String()
}

type fleet struct {
	t    *testing.T
	priv ed25519.PrivateKey
}

func (f fleet) Sign(_ context.Context, org, wallet string, n custody.Network, msg []byte) ([]byte, error) {
	if n != custody.Solana {
		f.t.Fatalf("Solana sweep asked for network %q", n)
	}
	if c, _ := n.Curve(); c != custody.Ed25519 {
		f.t.Fatalf("Solana asked for a %s signature", c)
	}
	// The message, not a digest of it. A 32-byte payload here would mean the
	// caller hashed, which is the defect this whole package guards.
	if len(msg) == 32 {
		f.t.Fatal("the signer was handed 32 bytes; Solana signs the message, not a digest")
	}
	return ed25519.Sign(f.priv, msg), nil
}

func TestNativeSweepEndToEnd(t *testing.T) {
	priv, from := keyed(t)
	c := (&cluster{t: t, lamports: 1_000_000_000, fee: 5000}).chain(t)

	sig, err := custody.Sweep(context.Background(), c, fleet{t, priv}, custody.Transfer{
		OrgID: "acme", WalletID: "w1", From: from, To: mintAddr,
	})
	if err != nil {
		t.Fatal(err)
	}
	if sig == "" {
		t.Fatal("no signature returned")
	}
}

func TestTokenSweepEndToEnd(t *testing.T) {
	priv, from := keyed(t)
	cl := &cluster{t: t, lamports: 10_000_000, token: "1000000000", fee: 5000, decimals: 6, ataExists: true}
	c := cl.chain(t)

	if _, err := custody.Sweep(context.Background(), c, fleet{t, priv}, custody.Transfer{
		OrgID: "acme", WalletID: "w1", From: from, To: mintAddr, Token: mintAddr,
	}); err != nil {
		t.Fatal(err)
	}
	if cl.simulated == "" {
		t.Fatal("broadcast without simulating first")
	}
	if cl.simulated != cl.sent {
		t.Fatal("simulated one transaction and sent a different one")
	}

	raw, err := base64.StdEncoding.DecodeString(cl.sent)
	if err != nil {
		t.Fatal(err)
	}
	msg := raw[65:]
	p := parse(t, msg)
	if len(p.ins) != 1 {
		t.Fatalf("built %d instructions; the destination already had a token account, so one transfer is enough", len(p.ins))
	}
	in := p.ins[0]
	if in.data[0] != transferChecked {
		t.Errorf("instruction opcode %d, want TransferChecked (%d)", in.data[0], transferChecked)
	}
	if got := binary.LittleEndian.Uint64(in.data[1:9]); got != 1_000_000_000 {
		t.Errorf("moved %d base units, want the whole balance 1000000000", got)
	}
	if in.data[9] != 6 {
		t.Errorf("declared %d decimals, want the mint's 6", in.data[9])
	}
	if !ed25519.Verify(priv.Public().(ed25519.PublicKey), msg, raw[1:65]) {
		t.Fatal("the broadcast transaction's signature does not cover its message")
	}
}

func TestTokenSweepCreatesAMissingDestinationAccount(t *testing.T) {
	priv, from := keyed(t)
	cl := &cluster{t: t, lamports: 10_000_000, token: "500", fee: 5000, decimals: 6, ataExists: false}
	c := cl.chain(t)
	if _, err := custody.Sweep(context.Background(), c, fleet{t, priv}, custody.Transfer{
		OrgID: "acme", WalletID: "w1", From: from, To: mintAddr, Token: mintAddr,
	}); err != nil {
		t.Fatal(err)
	}
	raw, _ := base64.StdEncoding.DecodeString(cl.sent)
	p := parse(t, raw[65:])
	if len(p.ins) != 2 {
		t.Fatalf("built %d instructions; a missing destination account needs creating first", len(p.ins))
	}
	if p.ins[0].data[0] != createIdempotent {
		t.Errorf("first instruction is %d, want CreateIdempotent (%d)", p.ins[0].data[0], createIdempotent)
	}
}

func TestTokenSweepRefusesWithoutRentOrFee(t *testing.T) {
	_, from := keyed(t)
	cl := &cluster{t: t, lamports: 1000, token: "500", fee: 5000, decimals: 6, ataExists: false}
	c := cl.chain(t)
	_, err := c.Draft(context.Background(), custody.Transfer{
		OrgID: "acme", WalletID: "w1", From: from, To: mintAddr, Token: mintAddr,
	})
	var noFee *custody.ErrNoFee
	if !asNoFee(err, &noFee) {
		t.Fatalf("want ErrNoFee, got %T: %v", err, err)
	}
	if noFee.Need.Int64() < ataRent {
		t.Errorf("Need is %s, which does not include the rent for the account it must open", noFee.Need)
	}
	t.Logf("refused: %v", err)
}

func TestEmptyIsNotAnError(t *testing.T) {
	_, from := keyed(t)
	c := (&cluster{t: t, lamports: 10_000_000, token: "0", fee: 5000, decimals: 6, ataExists: true}).chain(t)
	_, err := c.Draft(context.Background(), custody.Transfer{
		OrgID: "acme", WalletID: "w1", From: from, To: mintAddr, Token: mintAddr,
	})
	if err != custody.ErrEmpty {
		t.Fatalf("want ErrEmpty, got %v", err)
	}
}

func TestSealRefusesAForeignSignature(t *testing.T) {
	_, from := keyed(t)
	c := (&cluster{t: t, lamports: 1_000_000_000, fee: 5000}).chain(t)
	d, err := c.Draft(context.Background(), custody.Transfer{
		OrgID: "acme", WalletID: "w1", From: from, To: mintAddr,
	})
	if err != nil {
		t.Fatal(err)
	}
	other := ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize))
	if _, err := d.Seal([][]byte{ed25519.Sign(other, d.Digests[0])}); err == nil {
		t.Fatal("sealed a signature from a key that does not own the address")
	} else if !strings.Contains(err.Error(), "does not verify") {
		t.Fatalf("wrong refusal: %v", err)
	}
	if _, err := d.Seal([][]byte{make([]byte, 64)}); err == nil {
		t.Fatal("sealed an all-zero signature")
	}
	if _, err := d.Seal([][]byte{make([]byte, 65)}); err == nil {
		t.Fatal("sealed a 65-byte secp256k1-shaped signature")
	}
}

// TestBroadcastRefusesWhenSimulationFails: the cluster's own runtime is the
// last check, and a failure there must stop the send rather than annotate it.
func TestBroadcastRefusesWhenSimulationFails(t *testing.T) {
	priv, from := keyed(t)
	cl := &cluster{t: t, lamports: 1_000_000_000, fee: 5000, simErr: `"BlockhashNotFound"`}
	c := cl.chain(t)
	_, err := custody.Sweep(context.Background(), c, fleet{t, priv}, custody.Transfer{
		OrgID: "acme", WalletID: "w1", From: from, To: mintAddr,
	})
	if err == nil {
		t.Fatal("broadcast a transaction the cluster said would fail")
	}
	if cl.sent != "" {
		t.Fatal("sent it anyway")
	}
}

func asNoFee(err error, out **custody.ErrNoFee) bool {
	for err != nil {
		if e, ok := err.(*custody.ErrNoFee); ok {
			*out = e
			return true
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}

func hexOf(b []byte) string {
	const h = "0123456789abcdef"
	out := make([]byte, 0, len(b)*2)
	for _, c := range b {
		out = append(out, h[c>>4], h[c&0xf])
	}
	return string(out)
}

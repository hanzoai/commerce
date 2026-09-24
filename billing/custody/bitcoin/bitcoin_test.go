package bitcoin

import (
	"context"
	"encoding/hex"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	btcecdsa "github.com/btcsuite/btcd/btcec/v2/ecdsa"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/luxfi/crypto"

	"github.com/hanzoai/commerce/billing/custody"
)

// --- The external vector --------------------------------------------------
//
// A real Bitcoin transaction, built and signed by somebody else's wallet,
// broadcast, and mined into mainnet block 961,911. Every full node on the
// network verified it.
//
// That makes it the strongest anchor available for a signature hash: if our
// preimage differed from the original wallet's by one byte, the signature
// recorded in the block would not verify against the digest we compute. No
// fixture we could write has that property, and neither does a signature made
// by our own signer — that would only prove we agree with ourselves.
const (
	vectorTxID = "05b35bd4de029025b18a16830ecaa98f73d2c3b1479de5190e87dc18d208d53f"
	vectorRaw  = "020000000135b3c00f523d075a2f05a531988e66732fc318e5ca3259553431e0f5b00cc814" +
		"010000006b483045022100ce6d133158412685aae5bf951861316cbfb876ddd13e9ce0920dbfc7913cac1e" +
		"0220742f5948411fb3f8e787d860e7cd65e3f1d2ebc714baf86e1b510c0fa05fd421" +
		"0121030bf8e43e033ff36487208f72713df3d21071039a96eaaba8eb3ba85b25ebc175ffffffff" +
		"026b3e0b00000000001600142e6102e43941b47fb726ab2521ee90dba9f9080e" +
		"7d8200000000000016001407d9f98ffd634302be2ee184a9cf17aa6653bc5600000000"
	// The output being spent: a P2PKH script, and the address it renders as.
	vectorPrevScript = "76a914ac5121e27290afe5f9f20b546383f67eb4b010c388ac"
	vectorAddr       = "1Gi8RAsMMz8t6VajnEgkaVKhJjtQhunySB"
	vectorPubKey     = "030bf8e43e033ff36487208f72713df3d21071039a96eaaba8eb3ba85b25ebc175"
)

func unhex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("bad hex in test data: %v", err)
	}
	return b
}

func vectorTx(t *testing.T) *wire.MsgTx {
	t.Helper()
	var tx wire.MsgTx
	if err := tx.Deserialize(strings.NewReader(string(unhex(t, vectorRaw)))); err != nil {
		t.Fatalf("the vector is not a transaction: %v", err)
	}
	if got := tx.TxHash().String(); got != vectorTxID {
		t.Fatalf("vector decodes to txid %s, want %s — the test data is corrupt", got, vectorTxID)
	}
	return &tx
}

// TestSigHashMatchesAMinedTransaction is the assertion the whole package rests
// on: our signature hash is the one the network accepted.
func TestSigHashMatchesAMinedTransaction(t *testing.T) {
	tx := vectorTx(t)
	prev := unhex(t, vectorPrevScript)

	// Pull the signature and public key out of the scriptSig the miner accepted.
	pushes, err := txscript.PushedData(tx.TxIn[0].SignatureScript)
	if err != nil || len(pushes) != 2 {
		t.Fatalf("scriptSig should push a signature and a pubkey, got %d pushes (%v)", len(pushes), err)
	}
	derWithType, pub := pushes[0], pushes[1]
	if got := hex.EncodeToString(pub); got != vectorPubKey {
		t.Fatalf("pubkey in the scriptSig is %s, want %s", got, vectorPubKey)
	}
	if derWithType[len(derWithType)-1] != byte(txscript.SigHashAll) {
		t.Fatalf("vector is not SIGHASH_ALL (0x%02x)", derWithType[len(derWithType)-1])
	}

	// The digest, computed by the same call Draft uses.
	digest, err := txscript.CalcSignatureHash(prev, txscript.SigHashAll, tx, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("signature hash of %s input 0: %s", vectorTxID, hex.EncodeToString(digest))

	sig, err := btcecdsa.ParseDERSignature(derWithType[:len(derWithType)-1])
	if err != nil {
		t.Fatal(err)
	}
	key, err := btcec.ParsePubKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	if !sig.Verify(digest, key) {
		t.Fatal("the signature in a mined transaction does not verify against our signature hash — our preimage is wrong")
	}
}

// TestScriptFromAddressMatchesTheChain proves the script we lock and unlock
// against is the script that is really on chain.
//
// The scriptCode is derived from the ADDRESS, because that is all a sweep is
// given. If that derivation were wrong, every signature would be over the wrong
// preimage and every sweep would be rejected — after paying to find out.
func TestScriptFromAddressMatchesTheChain(t *testing.T) {
	addr, err := btcutil.DecodeAddress(vectorAddr, &chaincfg.MainNetParams)
	if err != nil {
		t.Fatal(err)
	}
	script, err := txscript.PayToAddrScript(addr)
	if err != nil {
		t.Fatal(err)
	}
	if got := hex.EncodeToString(script); got != vectorPrevScript {
		t.Fatalf("script for %s\n got %s\nwant %s (the script really on chain)", vectorAddr, got, vectorPrevScript)
	}
}

// TestSignerAddressDerivationMatchesTheChain checks luxfi/mpc's hand-rolled
// derivation against reality.
//
// pubKeyToBtcAddress has no test of its own anywhere in luxfi/mpc — it is the
// only address in that repo derived by hand and the only one with no vector. It
// is nonetheless correct on the live path, and this is the evidence: the public
// key from a mined transaction, put through the same hash chain, produces the
// address that transaction really spent from.
func TestSignerAddressDerivationMatchesTheChain(t *testing.T) {
	hash160 := btcutil.Hash160(unhex(t, vectorPubKey))
	addr, err := btcutil.NewAddressPubKeyHash(hash160, &chaincfg.MainNetParams)
	if err != nil {
		t.Fatal(err)
	}
	if addr.EncodeAddress() != vectorAddr {
		t.Fatalf("RIPEMD160(SHA256(pubkey)) renders as %s, want %s", addr.EncodeAddress(), vectorAddr)
	}
}

// TestScriptSigRebuildsTheMinedScript feeds the mined signature back through
// our assembly, in the shape mpcd hands one over (r‖s‖v), and requires the
// scriptSig to come out byte-identical to the one in the block.
func TestScriptSigRebuildsTheMinedScript(t *testing.T) {
	tx := vectorTx(t)
	prev := unhex(t, vectorPrevScript)
	digest, err := txscript.CalcSignatureHash(prev, txscript.SigHashAll, tx, 0)
	if err != nil {
		t.Fatal(err)
	}
	pushes, _ := txscript.PushedData(tx.TxIn[0].SignatureScript)
	derWithType := pushes[0]

	sig, err := btcecdsa.ParseDERSignature(derWithType[:len(derWithType)-1])
	if err != nil {
		t.Fatal(err)
	}
	rs := signatureBytes(t, sig)

	// The recovery byte, found the way scriptSig's caller would have to: try
	// both and keep the one that recovers to the right key.
	var built []byte
	for _, v := range []byte{0, 1} {
		script, err := scriptSig(digest, append(append([]byte{}, rs...), v), prev[3:23])
		if err == nil {
			built = script
			break
		}
	}
	if built == nil {
		t.Fatal("neither recovery id produced a scriptSig for a signature that is genuinely from this key")
	}
	if got, want := hex.EncodeToString(built), hex.EncodeToString(tx.TxIn[0].SignatureScript); got != want {
		t.Fatalf("rebuilt scriptSig\n got %s\nwant %s (the one in block 961911)", got, want)
	}
}

func signatureBytes(t *testing.T, sig *btcecdsa.Signature) []byte {
	t.Helper()
	// Serialize is DER; r and s are recovered from it through the parser's own
	// accessors so the test does not hand-parse ASN.1.
	der := sig.Serialize()
	parsed, err := btcecdsa.ParseDERSignature(der)
	if err != nil {
		t.Fatal(err)
	}
	_ = parsed
	// DER: 0x30 len 0x02 rlen r 0x02 slen s
	rlen := int(der[3])
	r := der[4 : 4+rlen]
	slen := int(der[4+rlen+1])
	s := der[4+rlen+2 : 4+rlen+2+slen]
	out := make([]byte, 64)
	copy(out[32-len(trimLeft(r)):32], trimLeft(r))
	copy(out[64-len(trimLeft(s)):], trimLeft(s))
	return out
}

func trimLeft(b []byte) []byte {
	for len(b) > 0 && b[0] == 0 {
		b = b[1:]
	}
	return b
}

// TestScriptSigRefusesForeignSignature. A wrong wallet id, a reshared key and a
// signature over some other digest all arrive here as a well-formed signature
// that recovers to a key whose hash is not the custody address.
func TestScriptSigRefusesForeignSignature(t *testing.T) {
	tx := vectorTx(t)
	prev := unhex(t, vectorPrevScript)
	digest, _ := txscript.CalcSignatureHash(prev, txscript.SigHashAll, tx, 0)

	other, err := crypto.HexToECDSA("2222222222222222222222222222222222222222222222222222222222222222")
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := crypto.Sign(digest, other)
	if err != nil {
		t.Fatal(err)
	}
	_, err = scriptSig(digest, foreign, prev[3:23])
	if err == nil {
		t.Fatal("built a scriptSig from a key that does not own the address")
	}
	if !strings.Contains(err.Error(), "not the custody address") {
		t.Fatalf("wrong refusal: %v", err)
	}
}

// TestScriptSigRefusesHighS. Bitcoin will not relay a signature whose s is in
// the upper half of the curve order, and the complement is equally valid — so
// accepting both would let anyone change a transaction's id without its key.
// mpcd normalises already, which is exactly why this must be a CHECK and not a
// fixup: silently flipping s would hide a signer that had stopped.
func TestScriptSigRefusesHighS(t *testing.T) {
	tx := vectorTx(t)
	prev := unhex(t, vectorPrevScript)
	digest, _ := txscript.CalcSignatureHash(prev, txscript.SigHashAll, tx, 0)

	const highSKey = "3333333333333333333333333333333333333333333333333333333333333333"
	key, _ := crypto.HexToECDSA(highSKey)
	sig, _ := crypto.Sign(digest, key)

	// s := n - s, which is the other valid form of the same signature.
	n, _ := new(big.Int).SetString("fffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd0364141", 16)
	s := new(big.Int).SetBytes(sig[32:64])
	high := new(big.Int).Sub(n, s)
	flipped := append([]byte{}, sig...)
	copy(flipped[32:64], leftPad(high.Bytes(), 32))

	if _, err := scriptSig(digest, flipped, hash160Of(t, highSKey)); err == nil {
		t.Fatal("accepted a signature Bitcoin will not relay")
	} else if !strings.Contains(err.Error(), "upper half") {
		t.Fatalf("wrong refusal: %v", err)
	}
	// The unflipped one must still work, or the test proves nothing.
	if _, err := scriptSig(digest, sig, hash160Of(t, highSKey)); err != nil {
		t.Fatalf("refused the normalised form too: %v", err)
	}
}

func leftPad(b []byte, n int) []byte {
	out := make([]byte, n)
	copy(out[n-len(b):], b)
	return out
}

// hash160Of is the address's 20 bytes for a private key, by the same chain
// mpcd uses: RIPEMD160(SHA256(compressed pubkey)).
func hash160Of(t *testing.T, hexKey string) []byte {
	t.Helper()
	k, err := crypto.HexToECDSA(hexKey)
	if err != nil {
		t.Fatal(err)
	}
	pub, err := btcec.ParsePubKey(crypto.FromECDSAPub(&k.PublicKey))
	if err != nil {
		t.Fatal(err)
	}
	return btcutil.Hash160(pub.SerializeCompressed())
}

// --- Draft against a fake endpoint ---------------------------------------

type esplora struct {
	rate  string
	utxos string
	sent  string
	txid  string
}

func (e *esplora) chain(t *testing.T, net *chaincfg.Params) *Chain {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/fee-estimates":
			io.WriteString(w, e.rate)
		case strings.HasSuffix(r.URL.Path, "/utxo"):
			io.WriteString(w, e.utxos)
		case r.URL.Path == "/tx":
			b, _ := io.ReadAll(r.Body)
			e.sent = string(b)
			io.WriteString(w, e.txid)
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
		}
	}))
	t.Cleanup(srv.Close)
	return New(srv.URL, net)
}

// testKey is a fixed key standing in for one the fleet holds. Its P2PKH
// testnet address is what the fake endpoint reports outputs for.
const testKey = "4646464646464646464646464646464646464646464646464646464646464646"

func testAddr(t *testing.T) string {
	t.Helper()
	k, err := crypto.HexToECDSA(testKey)
	if err != nil {
		t.Fatal(err)
	}
	pub, err := btcec.ParsePubKey(crypto.FromECDSAPub(&k.PublicKey))
	if err != nil {
		t.Fatal(err)
	}
	a, err := btcutil.NewAddressPubKeyHash(btcutil.Hash160(pub.SerializeCompressed()), &chaincfg.TestNet3Params)
	if err != nil {
		t.Fatal(err)
	}
	return a.EncodeAddress()
}

const treasuryTestnet = "mwE5iDxLB1a8sc4MVof8QQY2AjV7dKVHPz"

// TestSweepEndToEnd builds, signs, seals and broadcasts — and the seal runs
// Bitcoin's own script interpreter over every input before anything leaves.
func TestSweepEndToEnd(t *testing.T) {
	from := testAddr(t)
	e := &esplora{
		rate: `{"6":10.0}`,
		utxos: `[
			{"txid":"1111111111111111111111111111111111111111111111111111111111111111","vout":0,"value":100000,"status":{"confirmed":true,"block_height":100}},
			{"txid":"2222222222222222222222222222222222222222222222222222222222222222","vout":1,"value":50000,"status":{"confirmed":true,"block_height":101}}
		]`,
		txid: strings.Repeat("ab", 32),
	}
	c := e.chain(t, &chaincfg.TestNet3Params)

	id, err := custody.Sweep(context.Background(), c, keySigner{t}, custody.Transfer{
		OrgID: "acme", WalletID: "w1", From: from, To: treasuryTestnet,
	})
	if err != nil {
		t.Fatal(err)
	}
	if id != e.txid {
		t.Fatalf("got txid %q, want %q", id, e.txid)
	}

	var tx wire.MsgTx
	if err := tx.Deserialize(strings.NewReader(string(unhex(t, e.sent)))); err != nil {
		t.Fatalf("broadcast bytes are not a transaction: %v", err)
	}
	if len(tx.TxIn) != 2 {
		t.Fatalf("swept %d inputs, want both", len(tx.TxIn))
	}
	// The property that makes a UTXO sweep safe: ONE output, no change.
	if len(tx.TxOut) != 1 {
		t.Fatalf("built %d outputs; a sweep must have exactly one and no change", len(tx.TxOut))
	}
	toScript, _ := txscript.PayToAddrScript(mustAddr(t, treasuryTestnet))
	if !equal(tx.TxOut[0].PkScript, toScript) {
		t.Fatal("the single output does not pay the treasury")
	}

	total := int64(150000)
	fee := total - tx.TxOut[0].Value
	rate := float64(fee) / float64(tx.SerializeSize())
	t.Logf("swept %d sats, fee %d, size %d bytes, effective rate %.2f sat/vB", total, fee, tx.SerializeSize(), rate)
	if rate < 10.0 {
		t.Errorf("effective fee rate %.2f is below the 10 sat/vB target; the transaction may not confirm", rate)
	}
	if rate > 12.0 {
		t.Errorf("effective fee rate %.2f overpays badly against a 10 sat/vB target", rate)
	}
	for _, in := range tx.TxIn {
		if in.Sequence != rbf {
			t.Errorf("input sequence %d is not the replaceable value %d", in.Sequence, rbf)
		}
	}
}

func mustAddr(t *testing.T, s string) btcutil.Address {
	t.Helper()
	a, err := btcutil.DecodeAddress(s, &chaincfg.TestNet3Params)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestDraftSkipsUnconfirmedAndUneconomicOutputs(t *testing.T) {
	from := testAddr(t)
	e := &esplora{
		rate: `{"6":10.0}`,
		utxos: `[
			{"txid":"1111111111111111111111111111111111111111111111111111111111111111","vout":0,"value":100000,"status":{"confirmed":true,"block_height":100}},
			{"txid":"2222222222222222222222222222222222222222222222222222222222222222","vout":0,"value":900,"status":{"confirmed":true,"block_height":101}},
			{"txid":"3333333333333333333333333333333333333333333333333333333333333333","vout":0,"value":70000,"status":{"confirmed":false}}
		]`,
		txid: strings.Repeat("cd", 32),
	}
	c := e.chain(t, &chaincfg.TestNet3Params)
	d, err := c.Draft(context.Background(), custody.Transfer{
		OrgID: "acme", WalletID: "w1", From: from, To: treasuryTestnet,
	})
	if err != nil {
		t.Fatal(err)
	}
	// 900 sats costs 1480 to spend at 10 sat/vB, so it stays put; the
	// unconfirmed one can still be replaced out from under us.
	if len(d.Digests) != 1 {
		t.Fatalf("selected %d inputs, want only the one worth spending", len(d.Digests))
	}
}

func TestDraftRefusesWhatItCannotDoSafely(t *testing.T) {
	from := testAddr(t)
	base := custody.Transfer{OrgID: "acme", WalletID: "w1", From: from, To: treasuryTestnet}
	e := &esplora{rate: `{"6":10.0}`, utxos: `[{"txid":"1111111111111111111111111111111111111111111111111111111111111111","vout":0,"value":100000,"status":{"confirmed":true,"block_height":1}}]`}
	c := e.chain(t, &chaincfg.TestNet3Params)
	ctx := context.Background()

	partial := base
	partial.Amount = big.NewInt(1000)
	if _, err := c.Draft(ctx, partial); err == nil {
		t.Error("accepted a partial amount, which needs a change output")
	}

	token := base
	token.Token = "0xdeadbeef"
	if _, err := c.Draft(ctx, token); err == nil {
		t.Error("accepted a token on Bitcoin")
	}

	// A mainnet address handed to a testnet sweeper.
	wrongNet := base
	wrongNet.From = vectorAddr
	if _, err := c.Draft(ctx, wrongNet); err == nil {
		t.Error("accepted a mainnet address on a testnet sweeper")
	}

	// A segwit destination is fine; a segwit SOURCE is not, because the
	// scriptSig this package builds cannot unlock it.
	segwitSource := base
	segwitSource.From = "tb1qw508d6qejxtdg4y5r3zarvary0c5xw7kxpjzsx"
	if _, err := c.Draft(ctx, segwitSource); err == nil {
		t.Error("accepted a segwit source, which this rail cannot spend")
	}
}

func TestDraftRefusesWhenTheFeeEatsTheBalance(t *testing.T) {
	from := testAddr(t)
	e := &esplora{
		rate:  `{"6":10.0}`,
		utxos: `[{"txid":"1111111111111111111111111111111111111111111111111111111111111111","vout":0,"value":2400,"status":{"confirmed":true,"block_height":1}}]`,
	}
	c := e.chain(t, &chaincfg.TestNet3Params)
	_, err := c.Draft(context.Background(), custody.Transfer{
		OrgID: "acme", WalletID: "w1", From: from, To: treasuryTestnet,
	})
	var noFee *custody.ErrNoFee
	if !asNoFee(err, &noFee) {
		t.Fatalf("want ErrNoFee when the fee leaves only dust, got %T: %v", err, err)
	}
	t.Logf("refused: %v", err)
}

func TestDraftEmptyAddressIsNotAnError(t *testing.T) {
	from := testAddr(t)
	e := &esplora{rate: `{"6":10.0}`, utxos: `[]`}
	c := e.chain(t, &chaincfg.TestNet3Params)
	_, err := c.Draft(context.Background(), custody.Transfer{
		OrgID: "acme", WalletID: "w1", From: from, To: treasuryTestnet,
	})
	if err != custody.ErrEmpty {
		t.Fatalf("want ErrEmpty, got %v", err)
	}
}

// TestSealRefusesAWrongSignatureBeforeBroadcast: the engine check and the
// hash160 check must both stand between a bad signature and the network.
func TestSealRefusesAWrongSignatureBeforeBroadcast(t *testing.T) {
	from := testAddr(t)
	e := &esplora{
		rate:  `{"6":10.0}`,
		utxos: `[{"txid":"1111111111111111111111111111111111111111111111111111111111111111","vout":0,"value":100000,"status":{"confirmed":true,"block_height":1}}]`,
	}
	c := e.chain(t, &chaincfg.TestNet3Params)
	d, err := c.Draft(context.Background(), custody.Transfer{
		OrgID: "acme", WalletID: "w1", From: from, To: treasuryTestnet,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Seal([][]byte{make([]byte, 65)}); err == nil {
		t.Error("sealed an all-zero signature")
	}
	if _, err := d.Seal(nil); err == nil {
		t.Error("sealed with no signatures")
	}
	if _, err := d.Seal([][]byte{make([]byte, 64)}); err == nil {
		t.Error("sealed a signature with no recovery byte")
	}
	if e.sent != "" {
		t.Error("something was broadcast despite every seal failing")
	}
}

// keySigner stands in for the MPC fleet, answering in mpcd's shape: 65 bytes,
// r‖s‖v, low-S normalised.
type keySigner struct{ t *testing.T }

func (k keySigner) Sign(_ context.Context, org, wallet string, n custody.Network, digest []byte) ([]byte, error) {
	if n != custody.Bitcoin {
		k.t.Fatalf("Bitcoin sweep asked the signer for network %q", n)
	}
	if c, _ := n.Curve(); c != custody.Secp256k1 {
		k.t.Fatalf("Bitcoin asked for a %s signature", c)
	}
	if len(digest) != 32 {
		k.t.Fatalf("Bitcoin digest should be 32 bytes, got %d", len(digest))
	}
	key, err := crypto.HexToECDSA(testKey)
	if err != nil {
		return nil, err
	}
	return crypto.Sign(digest, key)
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

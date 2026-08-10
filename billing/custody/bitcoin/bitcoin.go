// Package bitcoin sweeps a Bitcoin custody address.
//
// Bitcoin is not "one more chain". It has no accounts and no balances: an
// address owns a set of unspent OUTPUTS, a spend consumes whole outputs, and
// every one of them needs its own signature over its own preimage. So this is
// the package that makes billing/custody's plural Digests earn their shape.
//
// LEGACY P2PKH, and that is not a preference. mpcd derives btc_address as
// base58check(0x00 ‖ RIPEMD160(SHA256(compressed pubkey))) at
// luxfi/mpc cmd/mpcd/main.go pubKeyToBtcAddress — a pay-to-pubkey-hash address,
// spent with a scriptSig and the pre-segwit signature hash. luxfi/mpc's own
// documentation says otherwise (docs/content/docs/bitcoin.mdx shows
// NewAddressWitnessPubKeyHash, NewAddressTaproot and CalcWitnessSigHash), and
// building from those pages produces a segwit spend for a legacy output, which
// no node will accept. The code is the source of truth here; the docs are wrong.
//
// A sweep takes EVERYTHING and pays it to one address, so there is no change
// output. That is the single largest safety property in this file: a change
// output is an address the transaction invents, and an invented address on a
// UTXO chain is how coins go somewhere nobody can reach.
package bitcoin

import (
	"context"
	"fmt"
	"math"
	"math/big"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/ecdsa"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/luxfi/crypto"

	"github.com/hanzoai/commerce/billing/bitcoinrpc"
	"github.com/hanzoai/commerce/billing/custody"
)

const (
	// feeTarget is how many blocks we are willing to wait. Six is the
	// conventional "confirmed" depth and the estimate is far cheaper than the
	// one- or two-block target; a sweep is not urgent.
	feeTarget = 6

	// dust is the smallest output worth creating, in satoshis.
	//
	// 546 is the relay limit for a P2PKH output: below it a node will not
	// forward the transaction at all, so the fee would be spent on something
	// that never travels. It is the highest of the common per-type limits, so
	// using it for every destination refuses slightly more than strictly
	// necessary — the safe direction.
	dust = 546

	// scriptSigSize is the largest a P2PKH input script gets: a push of the DER
	// signature plus its hash-type byte (at most 72), and a push of the
	// compressed public key (33).
	//
	// It is used to SIZE the transaction before the signatures exist. Deriving
	// the fee from an over-estimate makes the real fee rate slightly HIGHER
	// than the target, never lower, which is the direction that still confirms.
	scriptSigSize = 1 + 72 + 1 + 33

	// inputVSize is what one more input costs, and it decides which outputs are
	// worth spending at all.
	inputVSize = 32 + 4 + 1 + scriptSigSize + 4

	// maxInputs keeps the transaction inside the 100,000-vbyte standardness
	// limit with room to spare. An address with more unspent outputs than this
	// is swept oldest-first and again next time.
	maxInputs = 500

	// rbf marks every input replaceable (BIP-125). A sweep that gets stuck
	// behind a fee spike can then be re-signed at a higher rate instead of
	// sitting in a mempool until it is evicted.
	rbf = wire.MaxTxInSequenceNum - 2
)

// Chain sweeps one Bitcoin network through one Esplora endpoint.
type Chain struct {
	rpc *bitcoinrpc.Client
	net *chaincfg.Params
}

// New builds a sweeper against an Esplora API root.
//
// net is explicit because Bitcoin has no equivalent of eth_chainId to ask — and
// because mpcd cannot answer it either: pubKeyToBtcAddress hard-codes version
// byte 0x00, so the fleet publishes a mainnet "1…" string on every deployment,
// testnet included. The same key is the same key; only the version byte and the
// endpoint differ, so a testnet proof re-encodes the same hash160 under
// TestNet3Params and spends through a testnet Esplora.
func New(esploraURL string, net *chaincfg.Params) *Chain {
	return &Chain{rpc: bitcoinrpc.NewClient(esploraURL), net: net}
}

// Network is the code the signer resolves a curve from. Bitcoin is secp256k1,
// signed by the same CGGMP21 ceremony that signs for the EVM chains.
func (c *Chain) Network() custody.Network { return custody.Bitcoin }

// Draft selects outputs, sizes the transaction, computes the fee and returns
// one digest per input.
func (c *Chain) Draft(ctx context.Context, t custody.Transfer) (*custody.Draft, error) {
	if t.Token != "" {
		return nil, fmt.Errorf("bitcoin: there are no tokens on this rail; asked to sweep %q", t.Token)
	}
	// A partial spend needs a change output, and a change output is the one
	// thing this package refuses to invent. Sweeping everything is the whole
	// use case; if a partial spend is ever wanted, its change belongs back at
	// Transfer.From and that is a deliberate decision for someone to make, not
	// a default to fall into.
	if t.Amount != nil {
		return nil, fmt.Errorf("bitcoin: a sweep moves the whole balance; a partial amount would need a change output")
	}

	from, err := c.address(t.From)
	if err != nil {
		return nil, fmt.Errorf("bitcoin: from: %w", err)
	}
	// The scriptSig this package builds is a P2PKH scriptSig. Anything else at
	// From is an output shape we cannot unlock, and finding that out after
	// broadcasting is the expensive way.
	fromPKH, ok := from.(*btcutil.AddressPubKeyHash)
	if !ok {
		return nil, fmt.Errorf("bitcoin: %s is a %T; this rail spends pay-to-pubkey-hash outputs, which is what the signer mints", t.From, from)
	}
	to, err := c.address(t.To)
	if err != nil {
		return nil, fmt.Errorf("bitcoin: to: %w", err)
	}

	fromScript, err := txscript.PayToAddrScript(fromPKH)
	if err != nil {
		return nil, fmt.Errorf("bitcoin: script for %s: %w", t.From, err)
	}
	toScript, err := txscript.PayToAddrScript(to)
	if err != nil {
		return nil, fmt.Errorf("bitcoin: script for %s: %w", t.To, err)
	}

	rate, err := c.rpc.FeeRate(ctx, feeTarget)
	if err != nil {
		return nil, fmt.Errorf("bitcoin: fee rate: %w", err)
	}
	utxos, err := c.rpc.Unspent(ctx, t.From)
	if err != nil {
		return nil, fmt.Errorf("bitcoin: unspent outputs of %s: %w", t.From, err)
	}

	tx := wire.NewMsgTx(wire.TxVersion)
	var total int64
	for _, u := range utxos {
		// An unconfirmed output is real value, but the transaction that created
		// it can still be replaced, and spending it would make our sweep
		// disappear with it.
		if !u.Confirmed() {
			continue
		}
		// An output worth less than it costs to spend is left where it is.
		// Including it would make the sweep collect less than it started with,
		// which is a strange thing for a sweep to do.
		if u.Value <= int64(math.Ceil(float64(inputVSize)*rate)) {
			continue
		}
		if len(tx.TxIn) >= maxInputs {
			break
		}
		h, err := chainhash.NewHashFromStr(u.TxID)
		if err != nil {
			return nil, fmt.Errorf("bitcoin: %q is not a txid: %w", u.TxID, err)
		}
		in := wire.NewTxIn(wire.NewOutPoint(h, u.Vout), nil, nil)
		in.Sequence = rbf
		tx.AddTxIn(in)
		total += u.Value
	}
	if len(tx.TxIn) == 0 {
		return nil, custody.ErrEmpty
	}

	// One output, holding everything the inputs carried minus the fee. The
	// value is a placeholder until the size is known, because the size depends
	// on how many bytes the value itself encodes to — it does not, at any
	// legal value, but relying on that is how a fee ends up one byte short.
	tx.AddTxOut(wire.NewTxOut(total, toScript))

	// Size the transaction with signatures that do not exist yet, by filling in
	// scripts of the largest size a real one can reach.
	for _, in := range tx.TxIn {
		in.SignatureScript = make([]byte, scriptSigSize)
	}
	size := tx.SerializeSize()
	for _, in := range tx.TxIn {
		in.SignatureScript = nil
	}

	fee := int64(math.Ceil(float64(size) * rate))
	value := total - fee
	if value < dust {
		return nil, &custody.ErrNoFee{
			Network: custody.Bitcoin, Addr: t.From,
			Have: bigOf(total), Need: bigOf(fee + dust),
		}
	}
	tx.TxOut[0].Value = value

	return c.draft(tx, fromScript, fromPKH.Hash160()[:])
}

// draft turns a built transaction into the Draft that Sweep drives. It is
// separate from Draft so a test can hand it a transaction of its own and check
// the signature hashes without a live endpoint.
func (c *Chain) draft(tx *wire.MsgTx, fromScript []byte, hash160 []byte) (*custody.Draft, error) {
	digests := make([][]byte, len(tx.TxIn))
	for i := range tx.TxIn {
		// The pre-segwit signature hash: the input under signature carries the
		// script of the output it spends, every other input's script is
		// blanked, and the hash type is appended before the double SHA-256.
		// txscript does that blanking itself, which is why the finished
		// transaction can be passed here as well as an empty one.
		h, err := txscript.CalcSignatureHash(fromScript, txscript.SigHashAll, tx, i)
		if err != nil {
			return nil, fmt.Errorf("bitcoin: signature hash for input %d: %w", i, err)
		}
		digests[i] = h
	}

	return &custody.Draft{
		Digests: digests,
		Seal: func(sigs [][]byte) ([]byte, error) {
			if len(sigs) != len(tx.TxIn) {
				return nil, fmt.Errorf("bitcoin: %d inputs need %d signatures, got %d", len(tx.TxIn), len(tx.TxIn), len(sigs))
			}
			for i, sig := range sigs {
				script, err := scriptSig(digests[i], sig, hash160)
				if err != nil {
					return nil, fmt.Errorf("bitcoin: input %d: %w", i, err)
				}
				tx.TxIn[i].SignatureScript = script
			}

			// Run Bitcoin's own script interpreter over every input before this
			// leaves the process.
			//
			// Every other check here is a check of our arithmetic. This one
			// asks the rules themselves, with the same engine a full node runs,
			// whether the transaction we built actually unlocks the coins we
			// aimed at. A UTXO spend is not retryable in any useful sense — the
			// fee is gone and the outputs are consumed — so the cheapest
			// possible moment to learn it is wrong is here.
			for i := range tx.TxIn {
				vm, err := txscript.NewEngine(fromScript, tx, i, txscript.StandardVerifyFlags, nil, nil, 0,
					txscript.NewCannedPrevOutputFetcher(fromScript, 0))
				if err != nil {
					return nil, fmt.Errorf("bitcoin: input %d: building the script engine: %w", i, err)
				}
				if err := vm.Execute(); err != nil {
					return nil, fmt.Errorf("bitcoin: input %d does not unlock its output: %w", i, err)
				}
			}

			var buf writeBuf
			if err := tx.Serialize(&buf); err != nil {
				return nil, fmt.Errorf("bitcoin: serialising: %w", err)
			}
			return []byte(buf), nil
		},
	}, nil
}

// Broadcast submits the signed transaction.
func (c *Chain) Broadcast(ctx context.Context, raw []byte) (string, error) {
	return c.rpc.Send(ctx, raw)
}

// scriptSig turns the signer's answer into "<signature> <pubkey>", and REFUSES
// anything that does not belong to the address being spent.
//
// The public key is RECOVERED from the signature rather than looked up, which
// is what makes the check possible at all: an address carries only
// RIPEMD160(SHA256(pubkey)), so the only way to know whether a signature came
// from the key behind it is to derive the key from the signature and hash it.
// A wrong wallet id, a reshared key and a signature over some other digest all
// recover to a key whose hash is not this one, and all of them stop here.
func scriptSig(digest, sig, hash160 []byte) ([]byte, error) {
	if len(sig) != 65 {
		return nil, fmt.Errorf("expected a 65-byte r‖s‖v signature, got %d bytes", len(sig))
	}
	var r, s btcec.ModNScalar
	if overflow := r.SetByteSlice(sig[:32]); overflow {
		return nil, fmt.Errorf("signature r is not a valid scalar")
	}
	if overflow := s.SetByteSlice(sig[32:64]); overflow {
		return nil, fmt.Errorf("signature s is not a valid scalar")
	}
	if r.IsZero() || s.IsZero() {
		return nil, fmt.Errorf("signature has a zero component")
	}
	// Bitcoin refuses to relay a signature whose s is in the upper half of the
	// curve order (BIP-62): the same signature with the complement of s is
	// equally valid, so accepting both would let anyone change a transaction's
	// id without its key. mpcd already normalises — SigEthereum applies the
	// identical EIP-2 rule — so this is a check and not a fixup. Silently
	// flipping s here would hide a signer that had stopped normalising.
	if s.IsOverHalfOrder() {
		return nil, fmt.Errorf("signature s is in the upper half of the curve order; Bitcoin will not relay it")
	}

	pub, err := crypto.Ecrecover(digest, sig)
	if err != nil {
		return nil, fmt.Errorf("signature does not recover to any public key: %w", err)
	}
	key, err := btcec.ParsePubKey(pub)
	if err != nil {
		return nil, fmt.Errorf("recovered public key is not on the curve: %w", err)
	}
	compressed := key.SerializeCompressed()
	if got := btcutil.Hash160(compressed); !equal(got, hash160) {
		return nil, fmt.Errorf("signature belongs to the key for %x, not the custody address %x — it must not be broadcast",
			got, hash160)
	}

	der := append(ecdsa.NewSignature(&r, &s).Serialize(), byte(txscript.SigHashAll))
	return txscript.NewScriptBuilder().AddData(der).AddData(compressed).Script()
}

// address decodes and requires the network to match.
//
// btcutil.DecodeAddress checks the version byte against net, so a mainnet
// address handed to a testnet sweeper is refused here rather than producing a
// transaction against outputs that do not exist.
func (c *Chain) address(s string) (btcutil.Address, error) {
	a, err := btcutil.DecodeAddress(s, c.net)
	if err != nil {
		return nil, fmt.Errorf("%q is not a %s address: %w", s, c.net.Name, err)
	}
	if !a.IsForNet(c.net) {
		return nil, fmt.Errorf("%q is not a %s address", s, c.net.Name)
	}
	return a, nil
}

func equal(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// writeBuf is an io.Writer over a growing slice, so the serialised transaction
// needs no bytes.Buffer ceremony.
type writeBuf []byte

func (w *writeBuf) Write(p []byte) (int, error) {
	*w = append(*w, p...)
	return len(p), nil
}

var _ custody.Chain = (*Chain)(nil)

// bigOf lifts a satoshi count into the big.Int ErrNoFee reports in, so one
// error type serves a chain that counts in satoshis and a chain that counts in
// wei.
func bigOf(n int64) *big.Int { return big.NewInt(n) }

// Package solana sweeps a Solana custody address: SOL or an SPL token.
//
// Two things make Solana different from every other chain here.
//
// The address IS the public key. Solana account ids are the raw 32 bytes of an
// Ed25519 key, base58-encoded and nothing more, so verifying that a signature
// belongs to the address being spent needs no recovery and no lookup — it is
// ed25519.Verify against the address itself. That makes the check this package
// runs before broadcasting stronger than the equivalent anywhere else.
//
// And the signature covers the MESSAGE, not a digest of it. Ed25519 hashes
// internally, so a caller that pre-hashed would be signing a digest of a digest
// and no verifier on earth would accept it. mpcd knows this — its FROST path
// takes the message untouched — which is why billing/custody's Signer is
// defined over bytes rather than over a fixed-width hash.
package solana

import (
	"context"
	"crypto/ed25519"
	"encoding/binary"
	"fmt"
	"math/big"

	"github.com/hanzoai/commerce/billing/custody"
	"github.com/hanzoai/commerce/billing/solanarpc"
)

// systemProgram is the built-in program that moves the native coin. Its id is
// thirty-two zero bytes, which base58-encodes to a string of ones.
var systemProgram = solanarpc.PublicKey{}

const (
	// transferChecked is the SPL Token instruction that moves tokens while
	// verifying the mint and its decimals. The unchecked Transfer (3) would do
	// the same work with one fewer account and no check; the check is the point.
	transferChecked = 12

	// createIdempotent is the Associated Token Account instruction that makes
	// the destination's token account if it is missing and succeeds quietly if
	// another transaction made it first. A sweep runs unattended, so the
	// version that tolerates a race is the only sane one.
	createIdempotent = 1

	// systemTransfer is the System Program's transfer instruction index. It is
	// a u32, unlike the SPL programs' single byte.
	systemTransfer = 2

	// ataRent is what it costs to open a token account, in lamports. It is
	// demanded only when the destination has none, and it is spent — rent
	// exemption is a deposit the account holds, not a fee.
	ataRent = 2_039_280
)

// Chain sweeps one Solana cluster.
type Chain struct {
	rpcURL string
	rpc    *solanarpc.Client
}

// New builds a sweeper against a Solana JSON-RPC endpoint.
func New(rpcURL string) *Chain {
	// The mint is empty because every call this client makes is chain-level;
	// a token's own reads go through a client bound to that token.
	return &Chain{rpcURL: rpcURL, rpc: solanarpc.NewClient(rpcURL, solanarpc.PublicKey{})}
}

// Network is the code the signer resolves a curve from. Solana is Ed25519,
// signed by the FROST ceremony rather than the CGGMP21 one.
func (c *Chain) Network() custody.Network { return custody.Solana }

// Draft reads the cluster and returns the message that must be signed.
func (c *Chain) Draft(ctx context.Context, t custody.Transfer) (*custody.Draft, error) {
	from, err := solanarpc.ParsePublicKey(t.From)
	if err != nil {
		return nil, fmt.Errorf("solana: from: %w", err)
	}
	to, err := solanarpc.ParsePublicKey(t.To)
	if err != nil {
		return nil, fmt.Errorf("solana: to: %w", err)
	}

	blockhash, _, err := c.rpc.Blockhash(ctx)
	if err != nil {
		return nil, fmt.Errorf("solana: recent blockhash: %w", err)
	}
	hash, err := solanarpc.ParsePublicKey(blockhash)
	if err != nil {
		return nil, fmt.Errorf("solana: blockhash %q: %w", blockhash, err)
	}

	held, err := c.rpc.Lamports(ctx, from)
	if err != nil {
		return nil, fmt.Errorf("solana: lamports of %s: %w", t.From, err)
	}

	var msg *message
	var need uint64
	if t.Token != "" {
		msg, need, err = c.token(ctx, from, to, hash, t)
	} else {
		msg, need, err = c.native(ctx, from, to, hash, t, held)
	}
	if err != nil {
		return nil, err
	}
	if held < need {
		return nil, &custody.ErrNoFee{
			Network: custody.Solana, Addr: t.From,
			Have: new(big.Int).SetUint64(held), Need: new(big.Int).SetUint64(need),
		}
	}
	return c.draft(msg, from), nil
}

// token builds an SPL transfer of the whole balance, creating the destination's
// token account when it has none.
func (c *Chain) token(ctx context.Context, from, to, hash solanarpc.PublicKey, t custody.Transfer) (*message, uint64, error) {
	mint, err := solanarpc.ParsePublicKey(t.Token)
	if err != nil {
		return nil, 0, fmt.Errorf("solana: token: %w", err)
	}
	// Token-2022 mints derive under their own program id, and using the classic
	// program's id for one yields an address that exists nowhere. This rail
	// watches classic SPL mints, so that is what it spends; a Token-2022 mint
	// would need its program id threaded through here and is refused by the
	// derivation rather than silently mis-derived.
	src, err := solanarpc.AssociatedTokenAddress(from, solanarpc.TokenProgramID, mint)
	if err != nil {
		return nil, 0, fmt.Errorf("solana: source token account: %w", err)
	}
	dst, err := solanarpc.AssociatedTokenAddress(to, solanarpc.TokenProgramID, mint)
	if err != nil {
		return nil, 0, fmt.Errorf("solana: destination token account: %w", err)
	}

	amount := t.Amount
	if amount == nil {
		amount, err = c.rpc.TokenBalance(ctx, src)
		if err != nil {
			return nil, 0, fmt.Errorf("solana: balance of %s: %w", src, err)
		}
	}
	if amount.Sign() <= 0 {
		return nil, 0, custody.ErrEmpty
	}
	if !amount.IsUint64() {
		return nil, 0, fmt.Errorf("solana: %s is more than a token account can hold", amount)
	}
	decimals, err := c.rpc.DecimalsOf(ctx, mint)
	if err != nil {
		return nil, 0, fmt.Errorf("solana: decimals of %s: %w", t.Token, err)
	}
	exists, err := c.rpc.Exists(ctx, dst)
	if err != nil {
		return nil, 0, fmt.Errorf("solana: checking %s: %w", dst, err)
	}

	m := newMessage(hash)
	// The custody address signs and pays, so it is the first account and the
	// only writable signer.
	payer := m.add(from, true, true)
	source := m.add(src, false, true)
	dest := m.add(dst, false, true)
	mintIdx := m.add(mint, false, false)
	token := m.add(solanarpc.TokenProgramID, false, false)

	need := uint64(0)
	if !exists {
		owner := m.add(to, false, false)
		system := m.add(systemProgram, false, false)
		ata := m.add(solanarpc.ATAProgramID, false, false)
		m.instruct(ata, []uint8{payer, dest, owner, mintIdx, system, token}, []byte{createIdempotent})
		need += ataRent
	}

	data := make([]byte, 10)
	data[0] = transferChecked
	binary.LittleEndian.PutUint64(data[1:9], amount.Uint64())
	data[9] = byte(decimals)
	m.instruct(token, []uint8{source, mintIdx, dest, payer}, data)

	fee, err := c.fee(ctx, m)
	if err != nil {
		return nil, 0, err
	}
	return m, need + fee, nil
}

// native builds a transfer of the whole SOL balance minus the fee.
func (c *Chain) native(ctx context.Context, from, to, hash solanarpc.PublicKey, t custody.Transfer, held uint64) (*message, uint64, error) {
	m := newMessage(hash)
	payer := m.add(from, true, true)
	dest := m.add(to, false, true)
	system := m.add(systemProgram, false, false)

	// The amount is a placeholder while the fee is priced. It does not change
	// the message's LENGTH — a u64 is eight bytes at every value — so the price
	// is the same for the real amount.
	data := make([]byte, 12)
	binary.LittleEndian.PutUint32(data[0:4], systemTransfer)
	m.instruct(system, []uint8{payer, dest}, data)

	fee, err := c.fee(ctx, m)
	if err != nil {
		return nil, 0, err
	}

	amount := t.Amount
	if amount == nil {
		if held <= fee {
			return nil, fee + 1, nil
		}
		// Everything, leaving the account empty. A system account at zero
		// lamports is simply deallocated, which is the correct end state for a
		// deposit address that has been swept.
		amount = new(big.Int).SetUint64(held - fee)
	}
	if !amount.IsUint64() {
		return nil, 0, fmt.Errorf("solana: %s lamports is out of range", amount)
	}
	binary.LittleEndian.PutUint64(data[4:12], amount.Uint64())
	return m, amount.Uint64() + fee, nil
}

func (c *Chain) fee(ctx context.Context, m *message) (uint64, error) {
	fee, err := c.rpc.FeeFor(ctx, m.bytes())
	if err != nil {
		return 0, fmt.Errorf("solana: pricing the message: %w", err)
	}
	return fee, nil
}

// draft wraps a built message. It is separate from Draft so a test can hand it
// a message of its own and check the signing bytes without a live cluster.
func (c *Chain) draft(m *message, from solanarpc.PublicKey) *custody.Draft {
	msg := m.bytes()
	return &custody.Draft{
		// The message itself, not a hash of it: Ed25519 hashes internally, and
		// anything hashed here would be signed as a digest of a digest.
		Digests: [][]byte{msg},
		Seal: func(sigs [][]byte) ([]byte, error) {
			if len(sigs) != 1 {
				return nil, fmt.Errorf("solana: expected 1 signature, got %d", len(sigs))
			}
			sig := sigs[0]
			if len(sig) != ed25519.SignatureSize {
				return nil, fmt.Errorf("solana: signature should be %d bytes, got %d", ed25519.SignatureSize, len(sig))
			}
			// The address IS the public key, so this asks the question directly:
			// does this signature come from the key that owns the coins? A wrong
			// wallet id, a signer answering on secp256k1, and a message that
			// drifted between drafting and signing all fail here.
			if !ed25519.Verify(ed25519.PublicKey(from[:]), msg, sig) {
				return nil, fmt.Errorf("solana: signature does not verify against custody address %s — it is for some other key and must not be broadcast", from)
			}
			out := make([]byte, 0, 1+len(sig)+len(msg))
			out = append(out, 1) // one signature, as a compact-u16
			out = append(out, sig...)
			return append(out, msg...), nil
		},
	}
}

// Broadcast simulates first, then submits.
//
// The simulation is not belt and braces. It runs the transaction against the
// cluster's real state with signature verification on, which catches an expired
// blockhash, an account that is not what we assumed and a program that rejects
// the instruction — none of which any local check can see, and all of which
// cost a fee to discover by sending.
func (c *Chain) Broadcast(ctx context.Context, raw []byte) (string, error) {
	if err := c.rpc.Simulate(ctx, raw); err != nil {
		return "", fmt.Errorf("solana: refusing to broadcast: %w", err)
	}
	return c.rpc.Send(ctx, raw)
}

// --- message ---------------------------------------------------------------

// message builds a legacy Solana message.
//
// The account list is ordered by PRIVILEGE and the header counts the classes:
// writable signers, then read-only signers, then writable accounts, then
// read-only ones. An instruction refers to its accounts by index into that
// list, so the ordering is not presentation — it is what says which accounts a
// program may write to and which keys must have signed.
type message struct {
	hash  solanarpc.PublicKey
	keys  []solanarpc.PublicKey
	sign  []bool
	write []bool
	ins   []instruction
}

type instruction struct {
	program  uint8
	accounts []uint8
	data     []byte
}

func newMessage(hash solanarpc.PublicKey) *message { return &message{hash: hash} }

// add appends an account and returns its index, merging a repeat into the
// existing entry with the WIDER privileges.
//
// Merging matters: the same key can appear as both the fee payer and a
// transfer's authority, and listing it twice would produce a message the
// runtime rejects. Widening rather than overwriting matters just as much — an
// account added read-only and needed writable must end up writable, never the
// other way round.
func (m *message) add(k solanarpc.PublicKey, signer, writable bool) uint8 {
	for i, existing := range m.keys {
		if existing == k {
			m.sign[i] = m.sign[i] || signer
			m.write[i] = m.write[i] || writable
			return uint8(i)
		}
	}
	m.keys = append(m.keys, k)
	m.sign = append(m.sign, signer)
	m.write = append(m.write, writable)
	return uint8(len(m.keys) - 1)
}

func (m *message) instruct(program uint8, accounts []uint8, data []byte) {
	m.ins = append(m.ins, instruction{program: program, accounts: accounts, data: data})
}

// bytes serialises the message in the order the runtime requires.
func (m *message) bytes() []byte {
	// Sort into the four privilege classes. The accounts were added in that
	// order by construction, but sorting here rather than trusting it keeps the
	// callers free to add in whatever order reads best.
	order := make([]int, 0, len(m.keys))
	for _, class := range [][2]bool{{true, true}, {true, false}, {false, true}, {false, false}} {
		for i := range m.keys {
			if m.sign[i] == class[0] && m.write[i] == class[1] {
				order = append(order, i)
			}
		}
	}
	at := make([]uint8, len(m.keys))
	for newIdx, oldIdx := range order {
		at[oldIdx] = uint8(newIdx)
	}

	var signers, readonlySigners, readonlyOthers uint8
	for _, i := range order {
		if m.sign[i] {
			signers++
			if !m.write[i] {
				readonlySigners++
			}
		} else if !m.write[i] {
			readonlyOthers++
		}
	}

	out := []byte{signers, readonlySigners, readonlyOthers}
	out = shortvec(out, len(order))
	for _, i := range order {
		out = append(out, m.keys[i][:]...)
	}
	out = append(out, m.hash[:]...)
	out = shortvec(out, len(m.ins))
	for _, in := range m.ins {
		out = append(out, at[in.program])
		out = shortvec(out, len(in.accounts))
		for _, a := range in.accounts {
			out = append(out, at[a])
		}
		out = shortvec(out, len(in.data))
		out = append(out, in.data...)
	}
	return out
}

// shortvec appends Solana's compact-u16: seven bits at a time, low group first,
// with the high bit set on every group but the last.
func shortvec(out []byte, n int) []byte {
	for {
		b := byte(n & 0x7f)
		n >>= 7
		if n == 0 {
			return append(out, b)
		}
		out = append(out, b|0x80)
	}
}

var _ custody.Chain = (*Chain)(nil)

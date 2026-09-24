package solanarpc

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hanzoai/commerce/billing/depositwatch"
)

// commitment is the ONLY commitment this client ever reads at.
//
// It is the answer to "what is finality on Solana", and it is not a block depth.
// Solana does not bury a transaction under N blocks; it ROOTS one, and
// `finalized` means a supermajority of stake has rooted the block — the point
// past which reverting it means breaking the protocol's own safety assumption,
// not merely outspending the chain. `confirmed` (optimistic confirmation) is one
// vote round short of that and can still be dropped.
//
// Reading only at `finalized` means a transaction this client can SEE is already
// final. The confirmation-depth rule the watcher then applies on top
// (RequiredConfirmationsForChain) is pure margin inside already-final territory,
// not the thing making the credit safe. That is deliberate: it keeps one
// confirmation policy for every chain instead of a Solana-shaped exception, and
// it makes "credited, then reorged away" unreachable rather than unlikely.
const commitment = "finalized"

const (
	// signaturePageLimit is the RPC's own maximum page size.
	signaturePageLimit = 1000
	// maxSignaturePages bounds one address's history walk. Exceeding it is an
	// ERROR and not a truncation: stopping early would silently drop the oldest
	// deposits in the window, and the cursor would then advance past them
	// forever. A stalled asset is loud and recoverable; a missed deposit is not.
	maxSignaturePages = 25
)

// Client reads SPL token deposits for ONE mint over Solana JSON-RPC. Safe for
// concurrent use.
type Client struct {
	rpcURL string
	mint   PublicKey
	http   *http.Client
	id     atomic.Int64

	mu    sync.Mutex
	mint_ *mintAccount            // cached once read; a mint's decimals never change
	atas  map[PublicKey]PublicKey // owner → associated token account
}

// NewClient builds a read client for one (endpoint, mint).
func NewClient(rpcURL string, mint PublicKey) *Client {
	return &Client{
		rpcURL: rpcURL,
		mint:   mint,
		http:   &http.Client{Timeout: 30 * time.Second},
		atas:   map[PublicKey]PublicKey{},
	}
}

var _ depositwatch.Reader = (*Client)(nil)

// --- JSON-RPC plumbing ---

type rpcReq struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int64  `json:"id"`
	Method  string `json:"method"`
	Params  []any  `json:"params"`
}

type rpcResp struct {
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func (c *Client) call(ctx context.Context, method string, params []any) (json.RawMessage, error) {
	body, err := json.Marshal(rpcReq{JSONRPC: "2.0", ID: c.id.Add(1), Method: method, Params: params})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.rpcURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("solanarpc: %s: %w", method, err)
	}
	defer resp.Body.Close()
	var out rpcResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("solanarpc: %s decode: %w", method, err)
	}
	if out.Error != nil {
		return nil, fmt.Errorf("solanarpc: %s error %d: %s", method, out.Error.Code, out.Error.Message)
	}
	return out.Result, nil
}

// --- depositwatch.Reader ---

// BlockNumber returns the highest FINALIZED slot.
//
// Slots are Solana's monotonic chain position and stand in for a block number
// everywhere the watcher does arithmetic on one. They are not the same thing: a
// leader that misses its turn leaves a slot with no block in it, so a depth
// measured in slots is always at least the depth measured in blocks. That
// direction is the safe one — it waits longer, never less.
func (c *Client) BlockNumber(ctx context.Context) (uint64, error) {
	raw, err := c.call(ctx, "getSlot", []any{map[string]any{"commitment": commitment}})
	if err != nil {
		return 0, err
	}
	var slot uint64
	if err := json.Unmarshal(raw, &slot); err != nil {
		return 0, fmt.Errorf("solanarpc: getSlot: %w", err)
	}
	return slot, nil
}

// Decimals returns the mint account's OWN decimals.
//
// This is the read that makes a mis-scaled credit unrepresentable rather than
// unlikely: the scale comes from the account that defines the token, so a
// mistyped mint fails to parse as a mint instead of quietly crediting at the
// wrong power of ten.
func (c *Client) Decimals(ctx context.Context) (int, error) {
	m, err := c.mintAccount(ctx)
	if err != nil {
		return 0, err
	}
	return int(m.decimals), nil
}

// Symbol returns the token's on-chain symbol, from its Metaplex Token Metadata
// account.
//
// This is a WEAKER guarantee than the EVM path's symbol(), and the difference is
// worth stating: an ERC-20 contract answers symbol() itself, whereas an SPL mint
// carries no name at all — the symbol lives in a separate account owned by a
// third-party program and is mutable by whoever holds its update authority. So
// this cannot prove an adversary's mint is not USDC. What it does prove is that
// the address an operator configured is the token they think it is, which is the
// failure that actually happens.
//
// A mint with NO metadata account is refused rather than accepted unnamed: an
// unidentifiable token on a rail that credits at a fixed dollar peg is exactly
// the case to fail closed on.
func (c *Client) Symbol(ctx context.Context) (string, error) {
	md, err := MetadataAddress(c.mint)
	if err != nil {
		return "", err
	}
	acct, err := c.accountInfo(ctx, md)
	if err != nil {
		return "", err
	}
	if acct == nil {
		return "", fmt.Errorf("solanarpc: mint %s has no Metaplex metadata account (%s) — it carries no on-chain symbol and cannot be identified", c.mint, md)
	}
	if acct.owner != MetadataProgramID {
		return "", fmt.Errorf("solanarpc: metadata account %s is owned by %s, not the Token Metadata program", md, acct.owner)
	}
	return parseMetadataSymbol(acct.data, c.mint)
}

// TransfersTo returns the token that arrived at each of `owners` between
// fromSlot and toSlot inclusive.
//
// `owners` are the addresses the rail MINTED and showed the customer — not the
// accounts the tokens land in. Each is translated to its Associated Token
// Account here, and the owner is what comes back out, so the caller matches the
// address it stored and never has to know an ATA exists.
//
// Cost is one getSignaturesForAddress per owner plus one getTransaction per
// transaction found. Solana has no server-side filter over an address SET, so
// this is linear in watched addresses per pass and not, as on the EVM path,
// linear in chunks of a hundred.
func (c *Client) TransfersTo(ctx context.Context, owners []string, fromSlot, toSlot uint64) ([]depositwatch.Transfer, error) {
	if len(owners) == 0 || toSlot < fromSlot {
		return nil, nil
	}
	m, err := c.mintAccount(ctx)
	if err != nil {
		return nil, err
	}

	// ata → the owner string the caller gave us, so what comes back is exactly
	// what the caller stored.
	byATA := make(map[PublicKey]string, len(owners))
	for _, o := range owners {
		owner, err := ParsePublicKey(strings.TrimSpace(o))
		if err != nil {
			return nil, fmt.Errorf("solanarpc: watched deposit address %q is not a Solana address: %w", o, err)
		}
		ata, err := c.associatedTokenAccount(owner, m.tokenProgram)
		if err != nil {
			return nil, err
		}
		byATA[ata] = o
	}

	// One transaction can credit two watched addresses, so signatures are
	// collected into a set before any of them is fetched.
	slotOf := map[string]uint64{}
	for ata := range byATA {
		sigs, err := c.signatures(ctx, ata, fromSlot, toSlot)
		if err != nil {
			return nil, err
		}
		for sig, slot := range sigs {
			slotOf[sig] = slot
		}
	}
	sigs := make([]string, 0, len(slotOf))
	for s := range slotOf {
		sigs = append(sigs, s)
	}
	sort.Strings(sigs) // deterministic RPC order; two replicas do the same work

	var out []depositwatch.Transfer
	for _, sig := range sigs {
		got, err := c.transfersInTx(ctx, sig, slotOf[sig], byATA, m)
		if err != nil {
			return nil, err
		}
		out = append(out, got...)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Block != out[j].Block {
			return out[i].Block < out[j].Block
		}
		if out[i].TxHash != out[j].TxHash {
			return out[i].TxHash < out[j].TxHash
		}
		return out[i].EventIndex < out[j].EventIndex
	})
	return out, nil
}

// --- signatures ---

type sigEntry struct {
	Signature string `json:"signature"`
	Slot      uint64 `json:"slot"`
	Err       any    `json:"err"`
}

// signatures walks one token account's history back to fromSlot.
//
// getSignaturesForAddress answers newest-first, so the walk stops at the first
// entry older than the window: everything behind it is older still. Entries
// NEWER than toSlot are skipped rather than stopping the walk — the chain
// advances between the head read and this call, and those slots belong to the
// next pass.
func (c *Client) signatures(ctx context.Context, acct PublicKey, fromSlot, toSlot uint64) (map[string]uint64, error) {
	out := map[string]uint64{}
	before := ""
	for page := 0; ; page++ {
		if page >= maxSignaturePages {
			return nil, fmt.Errorf("solanarpc: %s has more than %d signatures in slots %d..%d — refusing to scan a partial history",
				acct, maxSignaturePages*signaturePageLimit, fromSlot, toSlot)
		}
		opts := map[string]any{"limit": signaturePageLimit, "commitment": commitment}
		if before != "" {
			opts["before"] = before
		}
		raw, err := c.call(ctx, "getSignaturesForAddress", []any{acct.String(), opts})
		if err != nil {
			return nil, err
		}
		var entries []sigEntry
		if err := json.Unmarshal(raw, &entries); err != nil {
			return nil, fmt.Errorf("solanarpc: decode signatures: %w", err)
		}
		if len(entries) == 0 {
			return out, nil
		}
		for _, e := range entries {
			if e.Slot > toSlot {
				continue
			}
			if e.Slot < fromSlot {
				return out, nil
			}
			if e.Err != nil {
				continue // a failed transaction moved no tokens
			}
			out[e.Signature] = e.Slot
		}
		if len(entries) < signaturePageLimit {
			return out, nil
		}
		before = entries[len(entries)-1].Signature
	}
}

// --- transactions ---

type tokenBalance struct {
	AccountIndex  int    `json:"accountIndex"`
	Mint          string `json:"mint"`
	Owner         string `json:"owner"`
	UITokenAmount struct {
		Amount   string `json:"amount"`
		Decimals int    `json:"decimals"`
	} `json:"uiTokenAmount"`
}

type txResult struct {
	Slot uint64 `json:"slot"`
	Meta *struct {
		Err               any            `json:"err"`
		PreTokenBalances  []tokenBalance `json:"preTokenBalances"`
		PostTokenBalances []tokenBalance `json:"postTokenBalances"`
	} `json:"meta"`
	Transaction struct {
		Message struct {
			AccountKeys []struct {
				Pubkey string `json:"pubkey"`
			} `json:"accountKeys"`
		} `json:"message"`
	} `json:"transaction"`
}

// transfersInTx turns ONE transaction into the credits it produced for the
// watched accounts.
//
// The amount is the token account's balance AFTER minus BEFORE, not an amount
// decoded from a transfer instruction. That is the honest number: it is what the
// account actually holds more of, so it survives a transaction that transfers
// twice into the same account, one that transfers in and out again, and a
// Token-2022 transfer fee that makes the amount received smaller than the amount
// sent. An account absent from preTokenBalances started at zero — which is the
// normal shape of a FIRST deposit, where the sender creates the ATA in the same
// transaction.
func (c *Client) transfersInTx(ctx context.Context, sig string, slot uint64, byATA map[PublicKey]string, m *mintAccount) ([]depositwatch.Transfer, error) {
	raw, err := c.call(ctx, "getTransaction", []any{sig, map[string]any{
		"encoding":                       "jsonParsed",
		"commitment":                     commitment,
		"maxSupportedTransactionVersion": 0,
	}})
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		// The signature came from this same node's finalized index, so a missing
		// transaction is a node that cannot answer for its own history — not an
		// absence. Refusing leaves the cursor parked and retries next pass.
		return nil, fmt.Errorf("solanarpc: transaction %s is indexed at slot %d but cannot be read", sig, slot)
	}
	var tx txResult
	if err := json.Unmarshal(raw, &tx); err != nil {
		return nil, fmt.Errorf("solanarpc: decode transaction %s: %w", sig, err)
	}
	if tx.Meta == nil {
		return nil, fmt.Errorf("solanarpc: transaction %s has no metadata — balances are unknown", sig)
	}
	if tx.Meta.Err != nil {
		return nil, nil // failed on chain; no tokens moved
	}

	keys := tx.Transaction.Message.AccountKeys
	pre := make(map[int]*big.Int, len(tx.Meta.PreTokenBalances))
	for i := range tx.Meta.PreTokenBalances {
		b := &tx.Meta.PreTokenBalances[i]
		amt, err := parseUnits(b.UITokenAmount.Amount)
		if err != nil {
			return nil, fmt.Errorf("solanarpc: %s pre-balance: %w", sig, err)
		}
		pre[b.AccountIndex] = amt
	}

	var out []depositwatch.Transfer
	for i := range tx.Meta.PostTokenBalances {
		b := &tx.Meta.PostTokenBalances[i]
		if b.AccountIndex < 0 || b.AccountIndex >= len(keys) {
			// The balance records index into the account list; if that list is
			// short we cannot say WHICH account changed. Guessing here is how a
			// deposit becomes invisible, so it is an error.
			return nil, fmt.Errorf("solanarpc: %s reports a balance for account %d but lists only %d accounts", sig, b.AccountIndex, len(keys))
		}
		acct, err := ParsePublicKey(keys[b.AccountIndex].Pubkey)
		if err != nil {
			return nil, fmt.Errorf("solanarpc: %s account %d: %w", sig, b.AccountIndex, err)
		}
		owner, watched := byATA[acct]
		if !watched {
			continue
		}
		// Three cross-checks on an account we ARE about to credit. Each one
		// catches a different way the index above could have resolved to the
		// wrong account, and together they make a mis-attributed or mis-scaled
		// credit unrepresentable rather than merely improbable.
		if b.Mint != c.mint.String() {
			return nil, fmt.Errorf("solanarpc: %s account %s holds mint %s, not %s", sig, acct, b.Mint, c.mint)
		}
		if b.Owner != "" && b.Owner != owner {
			return nil, fmt.Errorf("solanarpc: %s account %s is owned by %s, not by the watched address %s", sig, acct, b.Owner, owner)
		}
		if b.UITokenAmount.Decimals != int(m.decimals) {
			return nil, fmt.Errorf("solanarpc: %s reports %d decimals for mint %s, whose mint account says %d",
				sig, b.UITokenAmount.Decimals, c.mint, m.decimals)
		}

		post, err := parseUnits(b.UITokenAmount.Amount)
		if err != nil {
			return nil, fmt.Errorf("solanarpc: %s post-balance: %w", sig, err)
		}
		delta := new(big.Int).Sub(post, orZero(pre[b.AccountIndex]))
		if delta.Sign() <= 0 {
			continue // unchanged, or a withdrawal (a sweep of our own)
		}
		out = append(out, depositwatch.Transfer{
			To:     owner,
			Units:  delta,
			TxHash: sig,
			// The position of the balance record's account within this
			// transaction — Solana's analogue of an EVM log index. See
			// depositwatch.Transfer.EventIndex.
			EventIndex: uint64(b.AccountIndex),
			Block:      slot,
		})
	}
	return out, nil
}

func orZero(n *big.Int) *big.Int {
	if n == nil {
		return big.NewInt(0)
	}
	return n
}

// parseUnits reads a base-unit amount. The RPC renders it as a decimal STRING
// precisely because it does not fit a JSON number, so it is parsed as one.
func parseUnits(s string) (*big.Int, error) {
	n, ok := new(big.Int).SetString(strings.TrimSpace(s), 10)
	if !ok || n.Sign() < 0 {
		return nil, fmt.Errorf("%q is not a token amount", s)
	}
	return n, nil
}

// --- accounts ---

type account struct {
	owner PublicKey
	data  []byte
}

// accountInfo reads one account, returning (nil, nil) when it does not exist.
func (c *Client) accountInfo(ctx context.Context, key PublicKey) (*account, error) {
	raw, err := c.call(ctx, "getAccountInfo", []any{key.String(), map[string]any{
		"encoding": "base64", "commitment": commitment,
	}})
	if err != nil {
		return nil, err
	}
	var resp struct {
		Value *struct {
			Data  []string `json:"data"`
			Owner string   `json:"owner"`
		} `json:"value"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("solanarpc: decode account %s: %w", key, err)
	}
	if resp.Value == nil {
		return nil, nil
	}
	if len(resp.Value.Data) < 1 {
		return nil, fmt.Errorf("solanarpc: account %s returned no data", key)
	}
	data, err := base64.StdEncoding.DecodeString(resp.Value.Data[0])
	if err != nil {
		return nil, fmt.Errorf("solanarpc: account %s data: %w", key, err)
	}
	owner, err := ParsePublicKey(resp.Value.Owner)
	if err != nil {
		return nil, fmt.Errorf("solanarpc: account %s owner: %w", key, err)
	}
	return &account{owner: owner, data: data}, nil
}

// mintAccount is the parsed SPL mint: the two facts this rail needs from it.
type mintAccount struct {
	decimals     uint8
	tokenProgram PublicKey // classic SPL Token or Token-2022; decides ATA derivation
}

func (c *Client) mintAccount(ctx context.Context) (*mintAccount, error) {
	c.mu.Lock()
	cached := c.mint_
	c.mu.Unlock()
	if cached != nil {
		return cached, nil
	}
	acct, err := c.accountInfo(ctx, c.mint)
	if err != nil {
		return nil, err
	}
	if acct == nil {
		return nil, fmt.Errorf("solanarpc: mint %s does not exist", c.mint)
	}
	m, err := parseMint(acct, c.mint)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	c.mint_ = m
	c.mu.Unlock()
	return m, nil
}

// SPL mint layout: mint_authority COption<Pubkey> (36) | supply u64 (8) |
// decimals u8 | is_initialized bool | freeze_authority COption<Pubkey> (36).
const (
	mintLen             = 82
	mintDecimalsOffset  = 44
	mintInitialisedByte = 45
	// token2022TypeOffset is where Token-2022 puts its AccountType
	// discriminator: past the 165 bytes a token account occupies, precisely so
	// the two shapes can never be confused. 1 == Mint.
	token2022TypeOffset  = 165
	token2022AccountMint = 1
)

// parseMint reads an SPL mint, refusing anything that is not exactly one.
//
// The length checks are the load-bearing part. An SPL token ACCOUNT is 165
// bytes and is owned by the very same program, so "owned by the Token program
// and at least 82 bytes long" would happily accept a token account and read a
// byte of its owner pubkey as the decimals. Getting that wrong is a credit off
// by a random power of ten, so the shape is pinned exactly: a classic mint is 82
// bytes, and a Token-2022 mint is either 82 or padded past a token account's
// length and tagged as a mint at offset 165.
func parseMint(acct *account, key PublicKey) (*mintAccount, error) {
	switch acct.owner {
	case TokenProgramID:
		if len(acct.data) != mintLen {
			return nil, fmt.Errorf("solanarpc: %s is %d bytes, not an %d-byte SPL mint — it is not the token's mint account", key, len(acct.data), mintLen)
		}
	case Token2022ProgramID:
		switch {
		case len(acct.data) == mintLen:
		case len(acct.data) > token2022TypeOffset && acct.data[token2022TypeOffset] == token2022AccountMint:
		default:
			return nil, fmt.Errorf("solanarpc: %s is a %d-byte Token-2022 account that is not tagged as a mint", key, len(acct.data))
		}
	default:
		return nil, fmt.Errorf("solanarpc: %s is owned by %s, which is not an SPL token program", key, acct.owner)
	}
	if acct.data[mintInitialisedByte] != 1 {
		return nil, fmt.Errorf("solanarpc: mint %s is not initialised", key)
	}
	return &mintAccount{decimals: acct.data[mintDecimalsOffset], tokenProgram: acct.owner}, nil
}

// associatedTokenAccount derives and caches an owner's ATA. The derivation is a
// pure function of (owner, token program, mint) and the result never changes, so
// it costs one computation per deposit address for the life of the process and
// zero RPC calls — which matters when the alternative is asking the node for it
// on every pass, once per customer.
func (c *Client) associatedTokenAccount(owner, tokenProgram PublicKey) (PublicKey, error) {
	c.mu.Lock()
	if ata, ok := c.atas[owner]; ok {
		c.mu.Unlock()
		return ata, nil
	}
	c.mu.Unlock()

	ata, err := AssociatedTokenAddress(owner, tokenProgram, c.mint)
	if err != nil {
		return PublicKey{}, err
	}
	c.mu.Lock()
	c.atas[owner] = ata
	c.mu.Unlock()
	return ata, nil
}

// --- Metaplex Token Metadata ---

// metadataV1Key is the account discriminator Metaplex writes at byte 0.
const metadataV1Key = 4

// parseMetadataSymbol reads the symbol out of a Metaplex Token Metadata
// account and asserts the account describes the mint we asked about.
//
// Layout: key u8 | update_authority (32) | mint (32) | then Borsh strings
// name, symbol, uri — each a u32 little-endian length followed by that many
// bytes, NUL-padded to the field's maximum by the program that wrote it.
func parseMetadataSymbol(data []byte, mint PublicKey) (string, error) {
	if len(data) < 65 {
		return "", fmt.Errorf("solanarpc: metadata for %s is %d bytes, too short to hold a symbol", mint, len(data))
	}
	if data[0] != metadataV1Key {
		return "", fmt.Errorf("solanarpc: metadata for %s has discriminator %d, not %d", mint, data[0], metadataV1Key)
	}
	// The account address was derived FROM the mint, and it also names the mint;
	// checking both closes the gap where a derivation bug reads some other
	// token's name and blesses the wrong contract.
	var named PublicKey
	copy(named[:], data[33:65])
	if named != mint {
		return "", fmt.Errorf("solanarpc: metadata account describes mint %s, not %s", named, mint)
	}
	_, off, ok := borshString(data, 65) // name
	if !ok {
		return "", fmt.Errorf("solanarpc: metadata for %s has an unreadable name field", mint)
	}
	sym, _, ok := borshString(data, off)
	if !ok {
		return "", fmt.Errorf("solanarpc: metadata for %s has an unreadable symbol field", mint)
	}
	return strings.TrimSpace(strings.TrimRight(sym, "\x00")), nil
}

// borshString reads a length-prefixed string, returning the value, the offset
// just past it, and whether it was readable.
func borshString(data []byte, off int) (string, int, bool) {
	if off < 0 || off+4 > len(data) {
		return "", -1, false
	}
	n := int(binary.LittleEndian.Uint32(data[off : off+4]))
	off += 4
	// Metaplex caps name/symbol/uri at 32/10/200; anything claiming more than
	// the account holds is a layout we are not reading correctly.
	if n < 0 || off+n > len(data) {
		return "", -1, false
	}
	return string(data[off : off+n]), off + n, true
}

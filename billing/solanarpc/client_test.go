package solanarpc

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// ── real mainnet payloads ───────────────────────────────────────────────────
//
// Everything below was fetched from api.mainnet-beta.solana.com and pasted
// verbatim. Hand-written fixtures prove we can parse what we imagined; these
// prove we can parse what the cluster actually sends, which is the only version
// that will ever arrive in production.

// usdcMintData is the USDC mint account: 82 bytes, decimals 6 at offset 44.
const usdcMintData = "AQAAAJj+huiNm+Lqi8HMpIeLKYjCQPUrhCS/tA7Rot3LXhmbMW3vW6DFGwAGAQEAAABicKqKWcWUBbRShshncubNEm6bil06OFNtN/e0FOi2Zw=="

// usdcMetadataData is USDC's Metaplex Token Metadata account: name "USD Coin",
// symbol "USDC".
const usdcMetadataData = "BBzjWe1aAS4E+hQrnHUaHF6Hz9CgFhuchf/TG3jN/Nj2xvp6877brTo9ZfNqq8l0MbG75MLS9uDkfKYCA0UvXWEgAAAAVVNEIENvaW4AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAKAAAAVVNEQwAAAAAAAMgAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABAfwAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=="

// depositTxJSON is a real 15 USDC transfer on Solana mainnet, signature
// 2KQnbgfr7iQ6TR9CBygALZ5mjunDzA9bB9Uq4fcKV93asZLTfWZbyX8jFGqeMnegggMFQLnjyAZVBfKFPKBzKTFU,
// as getTransaction returns it. Account 1 pays, account 2 receives — so ONE
// recorded transaction exercises both directions of the balance delta.
const depositTxJSON = `{
 "slot": 437671790,
 "meta": {
  "err": null,
  "preTokenBalances": [
   {"accountIndex": 1, "mint": "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v", "owner": "GJRs4FwHtemZ5ZE9x3FNvJ8TMwitKTh21yxdRPqn7npE", "programId": "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA", "uiTokenAmount": {"amount": "904382420004", "decimals": 6, "uiAmount": 904382.420004, "uiAmountString": "904382.420004"}},
   {"accountIndex": 2, "mint": "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v", "owner": "8meoEbDNDAogUcAm88F5coEASwyLbqcAMr47WwhpukUx", "programId": "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA", "uiTokenAmount": {"amount": "999940", "decimals": 6, "uiAmount": 0.99994, "uiAmountString": "0.99994"}}
  ],
  "postTokenBalances": [
   {"accountIndex": 1, "mint": "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v", "owner": "GJRs4FwHtemZ5ZE9x3FNvJ8TMwitKTh21yxdRPqn7npE", "programId": "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA", "uiTokenAmount": {"amount": "904367420004", "decimals": 6, "uiAmount": 904367.420004, "uiAmountString": "904367.420004"}},
   {"accountIndex": 2, "mint": "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v", "owner": "8meoEbDNDAogUcAm88F5coEASwyLbqcAMr47WwhpukUx", "programId": "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA", "uiTokenAmount": {"amount": "15999940", "decimals": 6, "uiAmount": 15.99994, "uiAmountString": "15.99994"}}
  ]
 },
 "transaction": {"message": {"accountKeys": [
   {"pubkey": "GJRs4FwHtemZ5ZE9x3FNvJ8TMwitKTh21yxdRPqn7npE", "signer": true, "source": "transaction", "writable": true},
   {"pubkey": "DeqZejBFrRwWraY4g4besYmibTY1QcV1Fcg6VfoEvn4T", "signer": false, "source": "transaction", "writable": true},
   {"pubkey": "GLX7bTkwHg52vsqhZqg5h9C78Vg6G63tCqtD8JHCNmTo", "signer": false, "source": "transaction", "writable": true},
   {"pubkey": "ComputeBudget111111111111111111111111111111", "signer": false, "source": "transaction", "writable": false},
   {"pubkey": "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v", "signer": false, "source": "transaction", "writable": false},
   {"pubkey": "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA", "signer": false, "source": "transaction", "writable": false}
 ]}}
}`

const (
	depositSig  = "2KQnbgfr7iQ6TR9CBygALZ5mjunDzA9bB9Uq4fcKV93asZLTfWZbyX8jFGqeMnegggMFQLnjyAZVBfKFPKBzKTFU"
	depositSlot = 437671790
	// payee is the owner whose balance ROSE by 15 USDC; payer's fell by the same.
	payee = ownerBump253 // ATA GLX7bTkwHg52vsqhZqg5h9C78Vg6G63tCqtD8JHCNmTo, accountIndex 2
	payer = ownerBump255 // ATA DeqZejBFrRwWraY4g4besYmibTY1QcV1Fcg6VfoEvn4T, accountIndex 1
)

// systemProgram owns ordinary wallets — the thing an operator is most likely to
// paste where a mint belongs.
const systemProgram = "11111111111111111111111111111111"

// ── a Solana node that only answers what it was told ────────────────────────

type fakeAccount struct{ owner, data string }

type sigRow struct {
	Signature string `json:"signature"`
	Slot      uint64 `json:"slot"`
	Err       any    `json:"err"`
}

type fakeRPC struct {
	mu       sync.Mutex
	calls    []recordedCall
	slot     uint64
	accounts map[string]fakeAccount
	sigs     map[string][]sigRow // address → newest-first, as the RPC returns
	txs      map[string]string   // signature → raw getTransaction result
}

type recordedCall struct {
	method string
	params []json.RawMessage
}

func newFakeRPC() *fakeRPC {
	return &fakeRPC{
		slot:     depositSlot + 100,
		accounts: map[string]fakeAccount{},
		sigs:     map[string][]sigRow{},
		txs:      map[string]string{},
	}
}

// usdcNode is a node holding the real USDC mint, its real metadata, and the real
// deposit transaction, with that transaction indexed against BOTH participants'
// token accounts — exactly as getSignaturesForAddress reports it.
func usdcNode() *fakeRPC {
	f := newFakeRPC()
	f.accounts[usdcMint] = fakeAccount{owner: TokenProgramID.String(), data: usdcMintData}
	f.accounts[usdcMetadataPDA] = fakeAccount{owner: MetadataProgramID.String(), data: usdcMetadataData}
	f.txs[depositSig] = depositTxJSON
	row := sigRow{Signature: depositSig, Slot: depositSlot}
	f.sigs[ataBump253] = []sigRow{row}
	f.sigs[ataBump255] = []sigRow{row}
	return f
}

func (f *fakeRPC) start(t *testing.T) *Client {
	t.Helper()
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	return NewClient(srv.URL, mustKey(t, usdcMint))
}

func (f *fakeRPC) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Method string            `json:"method"`
		Params []json.RawMessage `json:"params"`
		ID     int64             `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	f.mu.Lock()
	f.calls = append(f.calls, recordedCall{method: req.Method, params: req.Params})
	f.mu.Unlock()

	write := func(result string) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":` + result + `}`))
	}
	arg := func(i int) string {
		var s string
		if i < len(req.Params) {
			_ = json.Unmarshal(req.Params[i], &s)
		}
		return s
	}

	switch req.Method {
	case "getSlot":
		write(jsonOf(f.slot))
	case "getAccountInfo":
		acct, ok := f.accounts[arg(0)]
		if !ok {
			write(`{"context":{"slot":1},"value":null}`)
			return
		}
		write(`{"context":{"slot":1},"value":{"data":[` + jsonOf(acct.data) + `,"base64"],"owner":` + jsonOf(acct.owner) + `,"lamports":1,"executable":false,"space":0}}`)
	case "getSignaturesForAddress":
		rows := f.sigs[arg(0)]
		var opts struct {
			Before string `json:"before"`
			Limit  int    `json:"limit"`
		}
		if len(req.Params) > 1 {
			_ = json.Unmarshal(req.Params[1], &opts)
		}
		if opts.Before != "" {
			rows = after(rows, opts.Before)
		}
		if opts.Limit > 0 && len(rows) > opts.Limit {
			rows = rows[:opts.Limit]
		}
		write(jsonOf(rows))
	case "getTransaction":
		tx, ok := f.txs[arg(0)]
		if !ok {
			write("null")
			return
		}
		write(tx)
	default:
		http.Error(w, "unexpected method "+req.Method, 500)
	}
}

func after(rows []sigRow, sig string) []sigRow {
	for i, r := range rows {
		if r.Signature == sig {
			return rows[i+1:]
		}
	}
	return nil
}

func jsonOf(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func (f *fakeRPC) methodCalls(method string) []recordedCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []recordedCall
	for _, c := range f.calls {
		if c.method == method {
			out = append(out, c)
		}
	}
	return out
}

// ── finality ────────────────────────────────────────────────────────────────

// Solana finality is a commitment level, not a block depth. Every read this
// client makes must ask for `finalized`, because a `confirmed` read can be
// dropped — and a deposit credited from a dropped block is money we gave away.
func TestEveryReadAsksForFinalizedCommitment(t *testing.T) {
	f := usdcNode()
	c := f.start(t)
	ctx := context.Background()

	if _, err := c.BlockNumber(ctx); err != nil {
		t.Fatalf("BlockNumber: %v", err)
	}
	if _, err := c.Symbol(ctx); err != nil {
		t.Fatalf("Symbol: %v", err)
	}
	if _, err := c.TransfersTo(ctx, []string{payee}, depositSlot-10, depositSlot+10); err != nil {
		t.Fatalf("TransfersTo: %v", err)
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.calls) == 0 {
		t.Fatal("no RPC calls were made")
	}
	for _, call := range f.calls {
		body := ""
		for _, p := range call.params {
			body += string(p)
		}
		if !strings.Contains(body, `"commitment":"finalized"`) {
			t.Fatalf("%s was called without finalized commitment: %s", call.method, body)
		}
	}
}

func TestBlockNumberReadsTheFinalizedSlot(t *testing.T) {
	f := usdcNode()
	f.slot = 437671999
	got, err := f.start(t).BlockNumber(context.Background())
	if err != nil {
		t.Fatalf("BlockNumber: %v", err)
	}
	if got != 437671999 {
		t.Fatalf("head = %d, want 437671999", got)
	}
}

// ── decimals come from the mint, never from us ──────────────────────────────

func TestDecimalsAreReadFromTheMintAccount(t *testing.T) {
	got, err := usdcNode().start(t).Decimals(context.Background())
	if err != nil {
		t.Fatalf("Decimals: %v", err)
	}
	if got != 6 {
		t.Fatalf("USDC decimals = %d, want 6", got)
	}
}

// The one that matters. An SPL token ACCOUNT is 165 bytes and is owned by the
// very same program as a mint, so "owned by the Token program" is not enough to
// call something a mint: byte 44 of a token account is a byte of its owner's
// public key, and reading it as decimals credits at a random power of ten. Each
// case here must be REFUSED, not guessed at.
func TestDecimalsRefuseAnythingThatIsNotAMint(t *testing.T) {
	for _, tc := range []struct {
		name, owner string
		data        []byte
		wantIn      string
	}{
		{
			name:  "an SPL token account, not the mint",
			owner: TokenProgramID.String(),
			data:  make([]byte, 165),
			// 165 bytes: exactly the shape that would otherwise sail through a
			// length >= 82 check and yield a byte of somebody's pubkey as decimals.
			wantIn: "not an 82-byte SPL mint",
		},
		{
			name:   "an ordinary wallet",
			owner:  systemProgram,
			data:   nil,
			wantIn: "not an SPL token program",
		},
		{
			name:   "an uninitialised mint",
			owner:  TokenProgramID.String(),
			data:   mintBytes(6, false),
			wantIn: "not initialised",
		},
		{
			name:   "a mint padded past its length",
			owner:  TokenProgramID.String(),
			data:   make([]byte, 200),
			wantIn: "not an 82-byte SPL mint",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := usdcNode()
			f.accounts[usdcMint] = fakeAccount{owner: tc.owner, data: b64(tc.data)}
			_, err := f.start(t).Decimals(context.Background())
			if err == nil {
				t.Fatal("accepted a non-mint account and would credit at whatever scale byte 44 happened to hold")
			}
			if !strings.Contains(err.Error(), tc.wantIn) {
				t.Fatalf("error %q does not mention %q", err, tc.wantIn)
			}
		})
	}
}

func TestDecimalsRefuseAMintThatDoesNotExist(t *testing.T) {
	f := usdcNode()
	delete(f.accounts, usdcMint)
	if _, err := f.start(t).Decimals(context.Background()); err == nil {
		t.Fatal("a mint address pointing at nothing was accepted")
	}
}

// A Token-2022 mint is padded past a token account's 165 bytes and tagged, so
// the same shape check works for both programs and the ATA derivation can pick
// up the right program id from here.
func TestDecimalsAcceptAToken2022Mint(t *testing.T) {
	data := make([]byte, 200)
	copy(data, mintBytes(9, true))
	data[165] = 1 // AccountType::Mint
	f := usdcNode()
	f.accounts[usdcMint] = fakeAccount{owner: Token2022ProgramID.String(), data: b64(data)}
	got, err := f.start(t).Decimals(context.Background())
	if err != nil {
		t.Fatalf("Decimals: %v", err)
	}
	if got != 9 {
		t.Fatalf("decimals = %d, want 9", got)
	}
}

// ── identity ────────────────────────────────────────────────────────────────

func TestSymbolReadsTheOnChainMetaplexSymbol(t *testing.T) {
	got, err := usdcNode().start(t).Symbol(context.Background())
	if err != nil {
		t.Fatalf("Symbol: %v", err)
	}
	if got != "USDC" {
		t.Fatalf("symbol = %q, want USDC", got)
	}
}

// An SPL mint has no name of its own, so a mint with no metadata account cannot
// be identified at all. On a rail that credits at a fixed dollar peg, that is
// the case to refuse rather than to accept unnamed.
func TestSymbolRefusesAMintWithNoMetadata(t *testing.T) {
	f := usdcNode()
	delete(f.accounts, usdcMetadataPDA)
	_, err := f.start(t).Symbol(context.Background())
	if err == nil {
		t.Fatal("a mint with no on-chain metadata was silently accepted")
	}
	if !strings.Contains(err.Error(), "cannot be identified") {
		t.Fatalf("error %q does not say the token is unidentifiable", err)
	}
}

// The metadata account is derived FROM the mint and also NAMES it. Checking
// both closes the gap where a derivation bug reads some other token's name and
// blesses the wrong mint.
func TestSymbolRefusesMetadataForADifferentMint(t *testing.T) {
	f := usdcNode()
	corrupted := decodeB64(t, usdcMetadataData)
	corrupted[33] ^= 0xff // flip a byte of the mint the metadata claims to describe
	f.accounts[usdcMetadataPDA] = fakeAccount{owner: MetadataProgramID.String(), data: b64(corrupted)}
	_, err := f.start(t).Symbol(context.Background())
	if err == nil {
		t.Fatal("accepted metadata describing a different mint")
	}
	if !strings.Contains(err.Error(), "describes mint") {
		t.Fatalf("error %q does not name the mismatch", err)
	}
}

func TestSymbolRefusesMetadataFromTheWrongProgram(t *testing.T) {
	f := usdcNode()
	f.accounts[usdcMetadataPDA] = fakeAccount{owner: systemProgram, data: usdcMetadataData}
	if _, err := f.start(t).Symbol(context.Background()); err == nil {
		t.Fatal("accepted a metadata account not owned by the Token Metadata program")
	}
}

// ── deposits ────────────────────────────────────────────────────────────────

// THE deposit test: an owner address we minted, an SPL transfer that landed in
// its derived ATA, and the amount taken from the balance delta.
func TestTransfersToCreditsTheBalanceDeltaOnTheDerivedATA(t *testing.T) {
	f := usdcNode()
	got, err := f.start(t).TransfersTo(context.Background(), []string{payee}, depositSlot-10, depositSlot+10)
	if err != nil {
		t.Fatalf("TransfersTo: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("saw %d transfers, want 1: %+v", len(got), got)
	}
	tr := got[0]
	if tr.To != payee {
		t.Fatalf("credited %q, want the OWNER address %q — the caller matches what it minted, not the token account", tr.To, payee)
	}
	if tr.Units.String() != "15000000" {
		t.Fatalf("amount = %s base units, want 15000000 (15 USDC at 6 decimals)", tr.Units)
	}
	if tr.TxHash != depositSig {
		t.Fatalf("txHash = %q, want the signature %q", tr.TxHash, depositSig)
	}
	if tr.EventIndex != 2 {
		t.Fatalf("event index = %d, want 2 — the position of the token-balance record within the transaction", tr.EventIndex)
	}
	if tr.Block != depositSlot {
		t.Fatalf("slot = %d, want %d", tr.Block, depositSlot)
	}
}

// The trap this whole package exists to avoid: an SPL transfer does not name
// the owner. Asking the node about the owner address returns nothing, forever,
// while the money sits in the ATA.
func TestTransfersToQueriesTheATAAndNeverTheOwner(t *testing.T) {
	f := usdcNode()
	if _, err := f.start(t).TransfersTo(context.Background(), []string{payee}, depositSlot-10, depositSlot+10); err != nil {
		t.Fatalf("TransfersTo: %v", err)
	}
	calls := f.methodCalls("getSignaturesForAddress")
	if len(calls) != 1 {
		t.Fatalf("made %d signature queries, want 1", len(calls))
	}
	var asked string
	_ = json.Unmarshal(calls[0].params[0], &asked)
	if asked == payee {
		t.Fatal("queried the OWNER address — an SPL transfer never names it, so every deposit would be invisible")
	}
	if asked != ataBump253 {
		t.Fatalf("queried %q, want the derived ATA %q", asked, ataBump253)
	}
}

// The same transaction, watched from the other side. The payer's balance FELL,
// and a falling balance is our own sweep — never a credit.
func TestTransfersToIgnoresAFallingBalance(t *testing.T) {
	f := usdcNode()
	got, err := f.start(t).TransfersTo(context.Background(), []string{payer}, depositSlot-10, depositSlot+10)
	if err != nil {
		t.Fatalf("TransfersTo: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("an outgoing transfer produced %d credits: %+v", len(got), got)
	}
}

// One transaction, two watched addresses, two DIFFERENT events. If the event
// index collapsed, one of these would be swallowed as a duplicate of the other.
func TestTransfersToDistinguishesTwoAccountsInOneTransaction(t *testing.T) {
	f := usdcNode()
	// Make the payer a receiver too, so both records are increases.
	f.txs[depositSig] = strings.Replace(depositTxJSON, `"amount": "904367420004"`, `"amount": "904392420004"`, 1)
	got, err := f.start(t).TransfersTo(context.Background(), []string{payee, payer}, depositSlot-10, depositSlot+10)
	if err != nil {
		t.Fatalf("TransfersTo: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("saw %d transfers, want 2: %+v", len(got), got)
	}
	if got[0].EventIndex == got[1].EventIndex {
		t.Fatalf("both events carry index %d — one deposit would be deduped away", got[0].EventIndex)
	}
	if got[0].TxHash != got[1].TxHash {
		t.Fatal("the two events should share a transaction id; only the event index distinguishes them")
	}
}

// A FIRST deposit normally creates the ATA in the same transaction, so the
// account has no pre-balance record at all. Reading that absence as anything but
// zero loses the customer's opening deposit.
func TestTransfersToTreatsAMissingPreBalanceAsZero(t *testing.T) {
	f := usdcNode()
	f.txs[depositSig] = strings.Replace(depositTxJSON,
		`{"accountIndex": 2, "mint": "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v", "owner": "8meoEbDNDAogUcAm88F5coEASwyLbqcAMr47WwhpukUx", "programId": "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA", "uiTokenAmount": {"amount": "999940", "decimals": 6, "uiAmount": 0.99994, "uiAmountString": "0.99994"}}`,
		`{"accountIndex": 9, "mint": "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v", "owner": "irrelevant", "programId": "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA", "uiTokenAmount": {"amount": "0", "decimals": 6, "uiAmount": 0.0, "uiAmountString": "0"}}`, 1)
	got, err := f.start(t).TransfersTo(context.Background(), []string{payee}, depositSlot-10, depositSlot+10)
	if err != nil {
		t.Fatalf("TransfersTo: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("saw %d transfers, want 1", len(got))
	}
	if got[0].Units.String() != "15999940" {
		t.Fatalf("amount = %s, want the whole post-balance 15999940 — a missing pre-balance is zero", got[0].Units)
	}
}

// Three cross-checks guard the account the balance record points at. Each one
// is a different way the index could resolve to the wrong account, and every
// one of them must REFUSE rather than credit somebody.
func TestTransfersToRefuseAnInconsistentBalanceRecord(t *testing.T) {
	for _, tc := range []struct {
		name, from, to, wantIn string
	}{
		{
			name:   "a different mint",
			from:   `{"accountIndex": 2, "mint": "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v", "owner": "8meoEbDNDAogUcAm88F5coEASwyLbqcAMr47WwhpukUx", "programId": "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA", "uiTokenAmount": {"amount": "15999940"`,
			to:     `{"accountIndex": 2, "mint": "Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB", "owner": "8meoEbDNDAogUcAm88F5coEASwyLbqcAMr47WwhpukUx", "programId": "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA", "uiTokenAmount": {"amount": "15999940"`,
			wantIn: "holds mint",
		},
		{
			name:   "a different owner",
			from:   `"owner": "8meoEbDNDAogUcAm88F5coEASwyLbqcAMr47WwhpukUx", "programId": "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA", "uiTokenAmount": {"amount": "15999940"`,
			to:     `"owner": "GJRs4FwHtemZ5ZE9x3FNvJ8TMwitKTh21yxdRPqn7npE", "programId": "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA", "uiTokenAmount": {"amount": "15999940"`,
			wantIn: "is owned by",
		},
		{
			// The mint says 6 decimals. A balance record claiming 9 means one of
			// the two is describing a different token, and crediting either way is
			// a 1000x error.
			name:   "decimals that disagree with the mint",
			from:   `{"amount": "15999940", "decimals": 6`,
			to:     `{"amount": "15999940", "decimals": 9`,
			wantIn: "decimals",
		},
		{
			name:   "an account index past the end of the account list",
			from:   `{"accountIndex": 2, "mint": "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v", "owner": "8meoEbDNDAogUcAm88F5coEASwyLbqcAMr47WwhpukUx", "programId": "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA", "uiTokenAmount": {"amount": "15999940"`,
			to:     `{"accountIndex": 99, "mint": "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v", "owner": "8meoEbDNDAogUcAm88F5coEASwyLbqcAMr47WwhpukUx", "programId": "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA", "uiTokenAmount": {"amount": "15999940"`,
			wantIn: "lists only",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := usdcNode()
			corrupted := strings.Replace(depositTxJSON, tc.from, tc.to, 1)
			if corrupted == depositTxJSON {
				t.Fatal("the fixture was not corrupted — this test proves nothing")
			}
			f.txs[depositSig] = corrupted
			_, err := f.start(t).TransfersTo(context.Background(), []string{payee}, depositSlot-10, depositSlot+10)
			if err == nil {
				t.Fatal("an inconsistent balance record was credited")
			}
			if !strings.Contains(err.Error(), tc.wantIn) {
				t.Fatalf("error %q does not mention %q", err, tc.wantIn)
			}
		})
	}
}

func TestTransfersToSkipsAFailedTransaction(t *testing.T) {
	f := usdcNode()
	f.txs[depositSig] = strings.Replace(depositTxJSON, `"err": null`, `"err": {"InstructionError": [0, "Custom"]}`, 1)
	got, err := f.start(t).TransfersTo(context.Background(), []string{payee}, depositSlot-10, depositSlot+10)
	if err != nil {
		t.Fatalf("TransfersTo: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("a failed transaction produced %d credits", len(got))
	}
}

// The signature index and the transaction store are the same node. A signature
// it indexed but cannot serve is a node that cannot answer for its own history;
// treating that as "no deposit" would let the cursor advance past real money.
func TestTransfersToRefuseAnUnreadableTransaction(t *testing.T) {
	f := usdcNode()
	delete(f.txs, depositSig)
	if _, err := f.start(t).TransfersTo(context.Background(), []string{payee}, depositSlot-10, depositSlot+10); err == nil {
		t.Fatal("a signature whose transaction could not be read was silently skipped")
	}
}

func TestTransfersToRefuseAWatchedAddressThatIsNotASolanaAddress(t *testing.T) {
	f := usdcNode()
	_, err := f.start(t).TransfersTo(context.Background(), []string{"0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed"}, 1, 10)
	if err == nil {
		t.Fatal("an EVM address was accepted as a Solana deposit address")
	}
}

// ── the scan window ─────────────────────────────────────────────────────────

// getSignaturesForAddress answers newest-first, so the walk stops at the first
// entry older than the window — and skips entries NEWER than it, because the
// chain advances between reading the head and asking this question.
func TestSignaturesHonourTheWindow(t *testing.T) {
	f := usdcNode()
	f.txs["older"] = strings.Replace(depositTxJSON, `"slot": 437671790`, `"slot": 1`, 1)
	f.txs["newer"] = strings.Replace(depositTxJSON, `"slot": 437671790`, `"slot": 999999999`, 1)
	f.sigs[ataBump253] = []sigRow{
		{Signature: "newer", Slot: depositSlot + 50}, // past toSlot
		{Signature: depositSig, Slot: depositSlot},   // inside
		{Signature: "failed", Slot: depositSlot - 1, Err: map[string]any{"x": 1}},
		{Signature: "older", Slot: depositSlot - 50}, // before fromSlot
	}
	got, err := f.start(t).TransfersTo(context.Background(), []string{payee}, depositSlot-10, depositSlot+10)
	if err != nil {
		t.Fatalf("TransfersTo: %v", err)
	}
	if len(got) != 1 || got[0].TxHash != depositSig {
		t.Fatalf("window not honoured; got %+v", got)
	}
	// The older signature is out of the window, so its transaction must never
	// have been fetched at all.
	for _, c := range f.methodCalls("getTransaction") {
		if strings.Contains(string(c.params[0]), "older") {
			t.Fatal("fetched a transaction from before the scan window")
		}
	}
}

// A history longer than the page cap is REFUSED, not truncated. Truncating
// would drop the oldest signatures in the window and the cursor would then
// advance past them — a deposit lost permanently and silently.
func TestSignaturesRefuseAPartialHistory(t *testing.T) {
	f := usdcNode()
	rows := make([]sigRow, 0, maxSignaturePages*signaturePageLimit+1)
	for i := 0; i <= maxSignaturePages*signaturePageLimit; i++ {
		rows = append(rows, sigRow{Signature: "sig" + jsonOf(i), Slot: depositSlot})
	}
	f.sigs[ataBump253] = rows
	_, err := f.start(t).TransfersTo(context.Background(), []string{payee}, depositSlot-10, depositSlot+10)
	if err == nil {
		t.Fatal("a history longer than the page cap was silently truncated")
	}
	if !strings.Contains(err.Error(), "partial history") {
		t.Fatalf("error %q does not say the history was partial", err)
	}
}

// A transaction touching two watched addresses is fetched ONCE. Two replicas
// and two re-scans must do identical work, and the signature set is what makes
// that true.
func TestTransfersToFetchesEachTransactionOnce(t *testing.T) {
	f := usdcNode()
	if _, err := f.start(t).TransfersTo(context.Background(), []string{payee, payer}, depositSlot-10, depositSlot+10); err != nil {
		t.Fatalf("TransfersTo: %v", err)
	}
	if n := len(f.methodCalls("getTransaction")); n != 1 {
		t.Fatalf("fetched the shared transaction %d times, want 1", n)
	}
}

// The ATA derivation is a pure function of (owner, program, mint), so it costs
// no RPC calls at all — which is what keeps a per-address chain affordable.
func TestATAsAreDerivedNotQueried(t *testing.T) {
	f := usdcNode()
	c := f.start(t)
	for i := 0; i < 3; i++ {
		if _, err := c.TransfersTo(context.Background(), []string{payee}, depositSlot-10, depositSlot+10); err != nil {
			t.Fatalf("pass %d: %v", i, err)
		}
	}
	if n := len(f.methodCalls("getTokenAccountsByOwner")); n != 0 {
		t.Fatal("asked the node to resolve an ATA that can be derived locally")
	}
	// The mint is read once and cached; its decimals cannot change.
	if n := len(f.methodCalls("getAccountInfo")); n != 1 {
		t.Fatalf("read the mint account %d times over 3 passes, want 1", n)
	}
}

// ── helpers ─────────────────────────────────────────────────────────────────

// mintBytes builds an 82-byte SPL mint with the given decimals.
func mintBytes(decimals byte, initialised bool) []byte {
	b := make([]byte, mintLen)
	b[mintDecimalsOffset] = decimals
	if initialised {
		b[mintInitialisedByte] = 1
	}
	return b
}

func b64(b []byte) string { return base64.StdEncoding.EncodeToString(b) }

func decodeB64(t *testing.T, s string) []byte {
	t.Helper()
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	return b
}

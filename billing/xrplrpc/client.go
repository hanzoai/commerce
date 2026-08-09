package xrplrpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hanzoai/commerce/billing/depositwatch"
)

// ledgerValidated is the ONLY ledger this client ever reads at, and it is the
// answer to "what is finality on XRPL".
//
// It is not a depth. XRPL does not bury a transaction under N ledgers and hope:
// a ledger is VALIDATED when a supermajority of the trusted validator set has
// declared it, and a validated ledger is final by construction of the consensus
// protocol — reverting one is not expensive, it is not a thing the protocol can
// do. This is much closer to Solana's `finalized` than to an EVM block depth,
// and the same consequence follows: a transaction this client can SEE is
// already final, and the confirmation depth the watcher then applies on top
// (RequiredConfirmationsForChain) is pure margin inside already-final
// territory. That keeps ONE confirmation policy for every chain instead of an
// XRPL-shaped exception, and makes "credited, then reorged away" unreachable
// rather than unlikely.
const ledgerValidated = "validated"

const (
	// accountTxPageLimit is one page of account_tx. rippled caps what it will
	// actually return; asking for more simply gets fewer.
	accountTxPageLimit = 200
	// maxAccountTxPages bounds one account's walk across a scan window.
	// Exceeding it is an ERROR and not a truncation, for the same reason it is
	// on Solana: stopping early would silently drop transactions from the
	// window and the cursor would then advance past them forever. A stalled
	// asset is loud and recoverable; a missed deposit is not.
	maxAccountTxPages = 50
)

// Client reads issued-currency deposits for ONE (currency, issuer) over XRPL
// JSON-RPC. Safe for concurrent use.
type Client struct {
	rpcURL string
	token  Issued
	http   *http.Client
	id     atomic.Int64

	mu     sync.Mutex
	symbol string // cached once read; a currency code's ticker never changes

	// amount decides what a delivered_amount MEANS for this reader, and it is
	// the only difference between reading an issued token and reading native
	// XRP. Everything else — paging by marker, the validated-ledger checks, the
	// tesSUCCESS rule, the destination tag — is identical and is shared rather
	// than written twice.
	amount func(m *txMeta, hash string) (*big.Int, bool, error)
	// decimals is the scale THIS reader's amounts are rendered at: Scale for an
	// issued currency, DropDecimals for native XRP.
	decimals int
	// native marks the reader as reading the ledger's own coin, which has no
	// issuer to ask about and therefore no Symbol round-trip.
	native bool
}

// NewClient builds a read client for one (endpoint, issued token).
func NewClient(rpcURL string, token Issued) *Client {
	c := &Client{
		rpcURL:   rpcURL,
		token:    token,
		http:     &http.Client{Timeout: 30 * time.Second},
		decimals: Scale,
	}
	c.amount = c.issuedAmount
	return c
}

var _ depositwatch.Reader = (*Client)(nil)

// --- JSON-RPC plumbing ---

type rpcReq struct {
	Method string `json:"method"`
	Params []any  `json:"params"`
	ID     int64  `json:"id"`
}

// call performs one XRPL JSON-RPC request.
//
// XRPL reports command failures INSIDE result — `{"result":{"status":"error",
// "error":"lgrNotFound"}}` — with HTTP 200 and no top-level error member. A
// client that only checked HTTP status and a top-level `error` would read a
// failed account_tx as an empty one, which is "this account received nothing"
// when the truth is "we could not look". That is a missed deposit, so the
// status member is checked on every response.
func (c *Client) call(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
	if params == nil {
		params = map[string]any{}
	}
	body, err := json.Marshal(rpcReq{Method: method, Params: []any{params}, ID: c.id.Add(1)})
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
		return nil, fmt.Errorf("xrplrpc: %s: %w", method, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("xrplrpc: %s: HTTP %d", method, resp.StatusCode)
	}
	var out struct {
		Result json.RawMessage `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("xrplrpc: %s decode: %w", method, err)
	}
	if len(out.Result) == 0 {
		return nil, fmt.Errorf("xrplrpc: %s returned no result", method)
	}
	var status struct {
		Status       string `json:"status"`
		Error        string `json:"error"`
		ErrorMessage string `json:"error_message"`
	}
	if err := json.Unmarshal(out.Result, &status); err != nil {
		return nil, fmt.Errorf("xrplrpc: %s decode status: %w", method, err)
	}
	if status.Error != "" || (status.Status != "" && status.Status != "success") {
		msg := status.ErrorMessage
		if msg == "" {
			msg = status.Error
		}
		return nil, fmt.Errorf("xrplrpc: %s: %s (%s)", method, msg, status.Error)
	}
	return out.Result, nil
}

// --- depositwatch.Reader ---

// BlockNumber returns the index of the latest VALIDATED ledger.
//
// A ledger index is a real global sequence — unlike a TON masterchain seqno it
// needs no argument for why, because XRPL has exactly one chain of ledgers and
// every transaction is in exactly one of them. The watcher's arithmetic over it
// (depth, window, commit-behind) is therefore literally true rather than an
// analogy.
func (c *Client) BlockNumber(ctx context.Context) (uint64, error) {
	raw, err := c.call(ctx, "ledger", map[string]any{"ledger_index": ledgerValidated})
	if err != nil {
		return 0, err
	}
	var resp struct {
		LedgerIndex uint64 `json:"ledger_index"`
		Validated   bool   `json:"validated"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return 0, fmt.Errorf("xrplrpc: ledger: %w", err)
	}
	if !resp.Validated {
		// We asked for the validated ledger and were handed one the server will
		// not call validated. Believing it would put the whole scan window on
		// ledgers that can still change.
		return 0, fmt.Errorf("xrplrpc: server answered ledger %d but does not report it validated", resp.LedgerIndex)
	}
	if resp.LedgerIndex == 0 {
		return 0, fmt.Errorf("xrplrpc: server reports validated ledger index 0")
	}
	return resp.LedgerIndex, nil
}

// Decimals returns the scale this client renders issued amounts at. See Scale —
// the number is a property of the parse, not a reading of the ledger, because
// an XRPL issued currency HAS no base unit to read.
func (c *Client) Decimals(context.Context) (int, error) { return c.decimals, nil }

// Symbol asks the LEDGER what the configured issuer actually issues, and
// returns the ticker of our configured currency code.
//
// The read is gateway_balances on the issuer, whose `obligations` are the
// currencies that issuer has outstanding. A configured pair that does not
// appear there is refused, and that refusal is the whole point: it catches the
// failure that actually happens — an issuer address off by a character, or a
// currency code from a different gateway — before a single deposit is credited
// at a dollar peg.
//
// This is a WEAKER guarantee than an ERC-20's symbol(), in the same way and for
// the same reason solanarpc's is: nothing stops an adversary from issuing a
// currency called USDC. It cannot prove the token is Circle's. What it proves
// is that the (currency, issuer) pair an operator configured is a live issued
// token on this ledger, which is the mistake that gets made.
func (c *Client) Symbol(ctx context.Context) (string, error) {
	// A native reader has no issuer to interrogate — XRP is the ledger's own
	// unit — so it answers without a round-trip that would ask about a zero
	// issuer and fail.
	if sym, ok := c.symbolFor(ctx); ok {
		return sym, nil
	}
	c.mu.Lock()
	cached := c.symbol
	c.mu.Unlock()
	if cached != "" {
		return cached, nil
	}

	raw, err := c.call(ctx, "gateway_balances", map[string]any{
		"account":      c.token.Issuer,
		"ledger_index": ledgerValidated,
	})
	if err != nil {
		return "", err
	}
	var resp struct {
		Obligations map[string]string `json:"obligations"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return "", fmt.Errorf("xrplrpc: gateway_balances: %w", err)
	}
	if len(resp.Obligations) == 0 {
		return "", fmt.Errorf("xrplrpc: account %s issues nothing on this ledger — it is not the issuer of %s", c.token.Issuer, c.token.Currency)
	}
	issues := make([]string, 0, len(resp.Obligations))
	for code := range resp.Obligations {
		cur, err := ParseCurrency(code)
		if err != nil {
			// A code we cannot parse is not a code we can match; record it for
			// the error below rather than failing on somebody else's token.
			issues = append(issues, code)
			continue
		}
		issues = append(issues, cur.String())
		if cur != c.token.Currency {
			continue
		}
		sym := cur.Symbol()
		if sym == "" {
			return "", fmt.Errorf("xrplrpc: currency %s carries no readable ticker — an unnameable token cannot be credited at a fixed peg", cur)
		}
		c.mu.Lock()
		c.symbol = sym
		c.mu.Unlock()
		return sym, nil
	}
	return "", fmt.Errorf("xrplrpc: account %s does not issue %s; it issues %s",
		c.token.Issuer, c.token.Currency, strings.Join(issues, ", "))
}

// TransfersTo returns the issued currency delivered to each of `addrs` in
// ledgers [fromLedger, toLedger].
//
// `addrs` are POOLED custody accounts — on XRPL there is normally exactly one,
// shared by every payer, because a funded account costs a non-refundable
// reserve. Which payer a delivery belongs to is carried by the destination tag,
// returned in Transfer.Tag, and matched against the intent by the policy half.
// A delivery carrying NO tag, or a tag naming no intent, still comes back from
// here: deciding it is unattributable is the watcher's call, not a filter that
// makes the money disappear from the read.
func (c *Client) TransfersTo(ctx context.Context, addrs []string, fromLedger, toLedger uint64) ([]depositwatch.Transfer, error) {
	if len(addrs) == 0 || toLedger < fromLedger {
		return nil, nil
	}
	var out []depositwatch.Transfer
	for _, addr := range addrs {
		got, err := c.deliveriesTo(ctx, strings.TrimSpace(addr), fromLedger, toLedger)
		if err != nil {
			return nil, err
		}
		out = append(out, got...)
	}
	return out, nil
}

// --- account_tx ---

type txEntry struct {
	Tx        *txJSON         `json:"tx"`
	Meta      *txMeta         `json:"meta"`
	Validated bool            `json:"validated"`
	Raw       json.RawMessage `json:"-"`
}

type txJSON struct {
	TransactionType string  `json:"TransactionType"`
	Account         string  `json:"Account"`
	Destination     string  `json:"Destination"`
	DestinationTag  *uint32 `json:"DestinationTag"`
	Hash            string  `json:"hash"`
	LedgerIndex     uint64  `json:"ledger_index"`
}

type txMeta struct {
	TransactionResult string          `json:"TransactionResult"`
	TransactionIndex  uint64          `json:"TransactionIndex"`
	DeliveredAmount   json.RawMessage `json:"delivered_amount"`
}

// amountObject is an issued-currency amount. XRP is rendered as a bare JSON
// string of drops instead, which is how the two are told apart.
type amountObject struct {
	Currency string `json:"currency"`
	Issuer   string `json:"issuer"`
	Value    string `json:"value"`
}

// deliveriesTo walks one account's validated transactions over the window and
// returns the deliveries of OUR token into it.
func (c *Client) deliveriesTo(ctx context.Context, addr string, fromLedger, toLedger uint64) ([]depositwatch.Transfer, error) {
	if _, err := ParseAccount(addr); err != nil {
		return nil, fmt.Errorf("xrplrpc: watched deposit address %q is not an XRPL account: %w", addr, err)
	}

	var out []depositwatch.Transfer
	var marker any
	for page := 0; ; page++ {
		if page >= maxAccountTxPages {
			return nil, fmt.Errorf("xrplrpc: %s has more than %d transactions in ledgers %d..%d — refusing to scan a partial window",
				addr, maxAccountTxPages*accountTxPageLimit, fromLedger, toLedger)
		}
		params := map[string]any{
			"account": addr,
			// The window is inclusive on both ends and is the server's own
			// filter, so nothing outside it is ever transferred or paged.
			"ledger_index_min": fromLedger,
			"ledger_index_max": toLedger,
			"binary":           false,
			"forward":          true, // ascending: deterministic order, stable paging
			"limit":            accountTxPageLimit,
			// Pin the response shape. XRPL API v2 renames tx→tx_json and moves
			// hash and ledger_index out of it; a server that silently answered
			// v2 would decode to an empty transaction. Pinned, the worst case is
			// a loud refusal in decodeEntry below.
			"api_version": 1,
		}
		if marker != nil {
			params["marker"] = marker
		}
		raw, err := c.call(ctx, "account_tx", params)
		if err != nil {
			return nil, err
		}
		var resp struct {
			Transactions []txEntry `json:"transactions"`
			Marker       any       `json:"marker"`
		}
		if err := json.Unmarshal(raw, &resp); err != nil {
			return nil, fmt.Errorf("xrplrpc: account_tx %s: %w", addr, err)
		}
		for i := range resp.Transactions {
			t, ok, err := c.delivery(&resp.Transactions[i], addr, fromLedger, toLedger)
			if err != nil {
				return nil, err
			}
			if ok {
				out = append(out, t)
			}
		}
		if resp.Marker == nil {
			return out, nil
		}
		marker = resp.Marker
	}
}

// delivery decides whether one account_tx entry is a deposit of our token into
// addr, and turns it into the watcher's chain-agnostic event.
//
// Only a Payment is ever a deposit here, and that narrowness is deliberate:
// Payment is the transaction that carries a DestinationTag AND delivers to
// exactly ONE destination, which together are what make (txHash) name the
// credit and let EventIndex be 0. Value can reach an account other ways —
// CheckCash, EscrowFinish, an offer being crossed — and none of them can route
// a payer's tag, so none of them is a customer's deposit. If a second kind is
// ever admitted here, EventIndex must stop being 0 and start being a real
// index, because two credits would then share a transaction.
func (c *Client) delivery(e *txEntry, addr string, fromLedger, toLedger uint64) (depositwatch.Transfer, bool, error) {
	var zero depositwatch.Transfer
	if e.Tx == nil || e.Meta == nil {
		return zero, false, fmt.Errorf("xrplrpc: account_tx %s returned an entry with no transaction or metadata — the server is answering a different API version", addr)
	}
	if !e.Validated {
		// account_tx was given a validated upper bound, so an unvalidated entry
		// means the server disagrees with itself about what is final.
		return zero, false, fmt.Errorf("xrplrpc: %s reports transaction %s as not validated inside a validated ledger range", addr, e.Tx.Hash)
	}
	if e.Tx.TransactionType != "Payment" || e.Tx.Destination != addr {
		return zero, false, nil
	}
	if e.Meta.TransactionResult != "tesSUCCESS" {
		return zero, false, nil // failed on ledger; nothing was delivered
	}
	if e.Tx.LedgerIndex < fromLedger || e.Tx.LedgerIndex > toLedger {
		return zero, false, fmt.Errorf("xrplrpc: %s answered ledger %d outside the requested window %d..%d", addr, e.Tx.LedgerIndex, fromLedger, toLedger)
	}

	units, ok, err := c.delivered(e.Meta, e.Tx.Hash)
	if err != nil || !ok {
		return zero, false, err
	}

	hash := strings.ToLower(strings.TrimSpace(e.Tx.Hash))
	if len(hash) != 64 {
		return zero, false, fmt.Errorf("xrplrpc: %s carries transaction hash %q, which is not a 256-bit hash", addr, e.Tx.Hash)
	}
	return depositwatch.Transfer{
		To:    addr,
		Tag:   destinationTag(e.Tx.DestinationTag),
		Units: units,
		// Lower-cased hex, ALWAYS. XRPL renders hashes uppercase, but the dedup
		// key is permanent and must not depend on a server's rendering choice —
		// a re-scan through anything that lower-cased them would otherwise
		// produce a second ledger row for the same deposit.
		TxHash: hash,
		// A Payment delivers to exactly one Destination with exactly one
		// DestinationTag, so its hash alone names this credit; 0 is the honest
		// "there is only one", not a placeholder. See the note on delivery.
		EventIndex: 0,
		Block:      e.Tx.LedgerIndex,
	}, true, nil
}

// delivered reads what the transaction ACTUALLY delivered, and reports whether
// it was our token.
//
// It reads meta.delivered_amount and NEVER tx.Amount, which is the single most
// important line in this file. With the tfPartialPayment flag set, Amount is
// the maximum the sender was willing to deliver and the ledger may deliver an
// arbitrarily smaller fraction of it — the classic XRPL partial-payment
// exploit, which has drained exchanges that credited Amount. delivered_amount
// is the ledger's own statement of what arrived.
//
// `unavailable` is refused rather than treated as zero. It appears on
// pre-2014 ledgers where the metadata cannot answer, and "we cannot tell how
// much arrived" must stop the pass, not silently credit nothing.
// delivered asks this reader's own amount rule what was delivered.
func (c *Client) delivered(m *txMeta, hash string) (*big.Int, bool, error) {
	return c.amount(m, hash)
}

// issuedAmount reads a delivered_amount as OUR issued token, and ignores
// everything else — including native XRP, which NewNative reads instead.
func (c *Client) issuedAmount(m *txMeta, hash string) (*big.Int, bool, error) {
	if len(m.DeliveredAmount) == 0 || string(m.DeliveredAmount) == "null" {
		return nil, false, nil // not a delivering payment
	}
	var s string
	if err := json.Unmarshal(m.DeliveredAmount, &s); err == nil {
		if s == "unavailable" {
			return nil, false, fmt.Errorf("xrplrpc: transaction %s does not report a delivered amount", hash)
		}
		// A bare string is DROPS of native XRP. XRP is not a dollar-pegged
		// token and this rail cannot price it, so it is simply not this asset —
		// exactly as ether sent to an EVM deposit address is not seen by the
		// USDC watcher.
		return nil, false, nil
	}
	var amt amountObject
	if err := json.Unmarshal(m.DeliveredAmount, &amt); err != nil {
		return nil, false, fmt.Errorf("xrplrpc: transaction %s has an unreadable delivered amount: %w", hash, err)
	}
	cur, err := ParseCurrency(amt.Currency)
	if err != nil {
		return nil, false, nil // some other gateway's token
	}
	// BOTH halves. A currency code is a name anyone may issue, so an issuer we
	// did not configure delivering "USDC" is a different — very possibly
	// worthless — token wearing our token's name.
	if cur != c.token.Currency || amt.Issuer != c.token.Issuer {
		return nil, false, nil
	}
	units, err := ParseValue(amt.Value)
	if err != nil {
		return nil, false, fmt.Errorf("xrplrpc: transaction %s: %w", hash, err)
	}
	if units.Sign() <= 0 {
		return nil, false, nil // nothing, or dust below the rendering scale
	}
	return units, true, nil
}

// destinationTag renders the tag that routes a pooled deposit to its intent.
//
// The empty string means the payment carried NO tag, and it is distinct from
// the tag "0" — 0 is a legal destination tag that some payer may genuinely have
// been issued. Collapsing the two would credit an untagged payment to whoever
// happened to hold tag 0.
func destinationTag(t *uint32) string {
	if t == nil {
		return ""
	}
	return strconv.FormatUint(uint64(*t), 10)
}

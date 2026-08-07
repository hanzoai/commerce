package tonrpc

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hanzoai/commerce/billing/depositwatch"
)

// opInternalTransfer is the jetton wallet's INBOUND credit message: the one a
// sibling jetton wallet sends to move tokens, and the only message whose
// successful processing means our balance went up.
//
// It is deliberately NOT op `transfer` (0x0f8a7ea5), which is the message the
// SENDER's wallet processes. The index records a jetton transfer against that
// sender-side transaction, and reading it would credit money that had merely
// been SENT: a jetton transfer can still fail at the destination — too little
// forward TON, a frozen or non-standard wallet — and bounce back to the sender
// with our customer's balance never having changed. This client therefore reads
// the receiving side only.
const opInternalTransfer = "0x178d4519"

// decodedInternalTransfer is what the index calls the decoded body of that
// message. It is asserted rather than assumed, so a decoder change downstream
// becomes a refusal instead of a misread amount.
const decodedInternalTransfer = "jetton_internal_transfer"

const (
	// transactionPageLimit is one page of an account's history.
	transactionPageLimit = 256
	// maxTransactionPages bounds one account's walk across a scan window.
	// Exceeding it is an ERROR and not a truncation, for the same reason it is
	// on Solana and XRPL: stopping early would silently drop the oldest
	// transactions in the window and the cursor would then advance past them
	// forever.
	maxTransactionPages = 40
)

// Client reads jetton deposits for ONE jetton master over the TON Index v3 HTTP
// API. Safe for concurrent use.
type Client struct {
	baseURL string
	master  Address
	http    *http.Client

	mu      sync.Mutex
	content *masterContent      // cached; a jetton's on-chain content is fixed in practice
	wallets map[Address]Address // owner → jetton wallet (derived on chain, never changes)
}

// NewClient builds a read client for one (TON Index v3 base URL, jetton
// master). The URL is the API BASE, e.g. https://toncenter.com/api/v3 — query
// parameters on it (a toncenter api_key, say) are preserved on every call.
func NewClient(baseURL string, master Address) *Client {
	return &Client{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		master:  master,
		http:    &http.Client{Timeout: 30 * time.Second},
		wallets: map[Address]Address{},
	}
}

var _ depositwatch.Reader = (*Client)(nil)

// --- HTTP plumbing ---

// get performs one TON Index request.
//
// EVERY caller of this VERIFIES THE RESPONSE AGAINST WHAT IT ASKED FOR rather
// than trusting the filter it sent, and that is not defensive habit — it is
// this API's measured behaviour. Several of its query parameters are SILENTLY
// IGNORED rather than rejected: `mc_seqno` on /jetton/transfers and
// `jetton_master` on /jetton/wallets both return HTTP 200 with completely
// unfiltered data. A filter that reads as success while filtering nothing is
// how a scan credits one customer's deposit to another, so nothing here relies
// on a filter having been applied.
func (c *Client) get(ctx context.Context, path string, params map[string]string, out any) error {
	u, err := url.Parse(c.baseURL + path)
	if err != nil {
		return fmt.Errorf("tonrpc: %s: %w", path, err)
	}
	q := u.Query()
	for k, v := range params {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("tonrpc: %s: %w", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("tonrpc: %s: HTTP %d", path, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("tonrpc: %s decode: %w", path, err)
	}
	return nil
}

// --- depositwatch.Reader ---

// BlockNumber returns the latest MASTERCHAIN seqno.
//
// THIS IS THE SCAN WINDOW, and TON is the one chain in this rail where the
// choice needs an argument, because TON has no single block height. The
// basechain is SHARDED and splits and merges under load, so a shard's own seqno
// is neither global nor comparable with another shard's, and a transaction's
// logical time (lt) — the other candidate — is a Lamport clock whose
// differences measure causality, not age, so `head − lt` is not a depth and
// arithmetic on it would be meaningless.
//
// The masterchain seqno is neither of those things. It is a single global
// sequence, it increments by one, and a transaction enters it exactly once:
// TON finalises a shard block by REFERENCING it from a masterchain block, so
// every transaction has exactly one masterchain seqno (mc_block_seqno) and it
// never changes afterwards. Depth beneath the head therefore means age, which
// is precisely the property depositwatch.Reader documents as the only one it
// relies on.
//
// It is the INDEXER's head, not the network's, and that is the right one: it is
// the newest position this client can actually read transactions at, so the
// window can never include blocks whose contents are not yet visible.
func (c *Client) BlockNumber(ctx context.Context) (uint64, error) {
	var resp struct {
		Last struct {
			Seqno     uint64 `json:"seqno"`
			Workchain int32  `json:"workchain"`
		} `json:"last"`
	}
	if err := c.get(ctx, "/masterchainInfo", nil, &resp); err != nil {
		return 0, err
	}
	if resp.Last.Seqno == 0 {
		return 0, fmt.Errorf("tonrpc: index reports masterchain seqno 0")
	}
	if resp.Last.Workchain != -1 {
		return 0, fmt.Errorf("tonrpc: masterchainInfo answered workchain %d, which is not the masterchain", resp.Last.Workchain)
	}
	return resp.Last.Seqno, nil
}

// Decimals returns the jetton's ON-CHAIN decimals.
//
// TEP-64 lets a jetton publish its metadata on chain (a dictionary in the
// master's content cell), off chain (a URI to a JSON document), or both. This
// reads the ON-CHAIN dictionary and NOTHING ELSE. A jetton that publishes its
// decimals only off chain is REFUSED, and that refusal is the point: the scale
// is what turns a deposit into cents, and taking it from an HTTP document the
// token issuer can rewrite — or that can simply be down — would make the
// arithmetic depend on a third party's web server rather than on consensus.
//
// USDT on TON does publish decimals on chain (6), which is what makes this
// workable rather than theoretical.
func (c *Client) Decimals(ctx context.Context) (int, error) {
	m, err := c.masterContent(ctx)
	if err != nil {
		return 0, err
	}
	raw := m.onChain("decimals")
	if raw == "" {
		return 0, fmt.Errorf("tonrpc: jetton master %s publishes no decimals in its on-chain content (it publishes %s) — the scale of a credit must not come from off-chain metadata",
			c.master, m.onChainKeys())
	}
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, fmt.Errorf("tonrpc: jetton master %s publishes decimals %q, which is not a number: %w", c.master, raw, err)
	}
	return n, nil
}

// Symbol returns the jetton's ON-CHAIN symbol, and refuses when there is none.
//
// ⚠ THIS REFUSES USDT ON TON, DELIBERATELY, AND THAT IS THE HONEST ANSWER.
// Tether's jetton master publishes only `decimals` and a `uri` on chain; its
// ticker lives in the off-chain JSON that URI points at, where it is written
// "USD₮" — with U+20AE, not the letter T. So there are exactly two ways to make
// the configured label "usdt" match, and both are worse than refusing:
//
//	fetch the URI    the identity of a token credited at a fixed dollar peg
//	                 would then be asserted by the issuer's web server, which
//	                 can change without any chain event and can be offline. A
//	                 boot-time money gate must not depend on that.
//	normalise ₮ to T  inventing a homoglyph table so a brand mark compares equal
//	                 to a ticker is a rule with no principle behind it, and the
//	                 next token would need a different one.
//
// So a jetton is armed here only if it says on chain what it is. See the note
// on Decimals for the same reasoning applied to the scale — with the difference
// that USDT does publish its decimals on chain, so only the identity check
// stops it.
func (c *Client) Symbol(ctx context.Context) (string, error) {
	m, err := c.masterContent(ctx)
	if err != nil {
		return "", err
	}
	if sym := strings.TrimSpace(m.onChain("symbol")); sym != "" {
		return sym, nil
	}
	off := ""
	if m.offChainSymbol != "" {
		off = fmt.Sprintf("; the index resolved its OFF-chain metadata to symbol %q, which this rail will not accept as identity", m.offChainSymbol)
	}
	return "", fmt.Errorf("tonrpc: jetton master %s publishes no symbol in its on-chain content (it publishes %s) — a token that does not say on chain what it is cannot be credited at a fixed dollar peg%s",
		c.master, m.onChainKeys(), off)
}

// TransfersTo returns the jetton that arrived at each of `owners` in masterchain
// blocks [fromSeqno, toSeqno] inclusive.
//
// `owners` are the addresses the rail MINTED and showed the customer — not the
// jetton wallets the tokens land in. Each is resolved to its jetton wallet here
// (the TON analogue of a Solana ATA: a separate contract, deterministically
// derived from owner and master, created by the first deposit itself), and the
// OWNER is what comes back out, spelled exactly as the caller spelled it, so
// the caller matches the address it stored and never has to know a jetton
// wallet exists.
func (c *Client) TransfersTo(ctx context.Context, owners []string, fromSeqno, toSeqno uint64) ([]depositwatch.Transfer, error) {
	if len(owners) == 0 || toSeqno < fromSeqno {
		return nil, nil
	}
	// A TON account has several correct spellings, so two watched addresses can
	// be the SAME account written differently — a case that cannot arise on
	// Solana, where base58 is canonical. Detected here and refused, because
	// "which of these two intents owns this account?" has no safe answer.
	byAccount := make(map[Address]string, len(owners))
	for _, o := range owners {
		addr, err := ParseAddress(o)
		if err != nil {
			return nil, fmt.Errorf("tonrpc: watched deposit address %q is not a TON address: %w", o, err)
		}
		if prev, dup := byAccount[addr]; dup {
			return nil, fmt.Errorf("tonrpc: watched addresses %q and %q are the same account (%s) written two ways", prev, o, addr)
		}
		byAccount[addr] = o
	}
	accounts := make([]Address, 0, len(byAccount))
	for a := range byAccount {
		accounts = append(accounts, a)
	}
	sort.Slice(accounts, func(i, j int) bool { return accounts[i].String() < accounts[j].String() })

	var out []depositwatch.Transfer
	for _, owner := range accounts {
		wallet, ok, err := c.jettonWallet(ctx, owner)
		if err != nil {
			return nil, err
		}
		if !ok {
			// No jetton wallet exists yet. On TON that is the normal state of an
			// address that has never been paid: the wallet contract is deployed
			// by the first transfer into it, exactly as a Solana ATA is. Nothing
			// to read, and emphatically not an error.
			continue
		}
		got, err := c.receipts(ctx, wallet, byAccount[owner], fromSeqno, toSeqno)
		if err != nil {
			return nil, err
		}
		out = append(out, got...)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Block != out[j].Block {
			return out[i].Block < out[j].Block
		}
		return out[i].TxHash < out[j].TxHash
	})
	return out, nil
}

// --- the jetton master's on-chain content ---

// masterContent is the jetton master's TEP-64 content, split by WHERE each
// value came from. The split is the whole value of this type: everything under
// `chain` was written to the ledger by the token, and everything else was
// fetched from a URI by the indexer.
type masterContent struct {
	chain          map[string]string
	offChainSymbol string
}

func (m *masterContent) onChain(key string) string { return m.chain[key] }

// onChainKeys renders what the token DOES publish on chain, so a refusal names
// the gap instead of just asserting one.
func (m *masterContent) onChainKeys() string {
	if len(m.chain) == 0 {
		return "nothing"
	}
	keys := make([]string, 0, len(m.chain))
	for k := range m.chain {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, ", ")
}

func (c *Client) masterContent(ctx context.Context) (*masterContent, error) {
	c.mu.Lock()
	cached := c.content
	c.mu.Unlock()
	if cached != nil {
		return cached, nil
	}

	var resp struct {
		JettonMasters []struct {
			Address       string         `json:"address"`
			JettonContent map[string]any `json:"jetton_content"`
		} `json:"jetton_masters"`
		Metadata map[string]struct {
			TokenInfo []struct {
				Symbol string `json:"symbol"`
			} `json:"token_info"`
		} `json:"metadata"`
	}
	if err := c.get(ctx, "/jetton/masters", map[string]string{"address": c.master.String()}, &resp); err != nil {
		return nil, err
	}

	m := &masterContent{chain: map[string]string{}}
	found := false
	for _, jm := range resp.JettonMasters {
		// Verify rather than trust the filter (see get).
		addr, err := ParseAddress(jm.Address)
		if err != nil || addr != c.master {
			continue
		}
		found = true
		for k, v := range jm.JettonContent {
			// TEP-64 on-chain values are strings. Anything else is a shape we do
			// not understand, and we would rather have no value than a coerced
			// one.
			if s, ok := v.(string); ok {
				m.chain[k] = s
			}
		}
	}
	if !found {
		return nil, fmt.Errorf("tonrpc: %s is not an indexed jetton master on this endpoint", c.master)
	}
	if md, ok := resp.Metadata[c.master.String()]; ok && len(md.TokenInfo) > 0 {
		m.offChainSymbol = md.TokenInfo[0].Symbol
	}

	c.mu.Lock()
	c.content = m
	c.mu.Unlock()
	return m, nil
}

// --- owner → jetton wallet ---

// jettonWallet resolves and caches an owner's jetton wallet for this master,
// reporting (_, false, nil) when no wallet exists yet.
//
// The address is a pure function of (owner, master) on chain, so it never
// changes and one resolution serves the life of the process — but the
// derivation needs the master's wallet CODE and a TVM cell hash, so it is asked
// of the index rather than computed here. The answer is then CHECKED against
// what was asked (see get): both `owner` and `jetton` on the returned record
// must match, because `/jetton/wallets` silently ignores an unrecognised filter
// parameter and would otherwise hand back some stranger's wallet.
func (c *Client) jettonWallet(ctx context.Context, owner Address) (Address, bool, error) {
	c.mu.Lock()
	w, ok := c.wallets[owner]
	c.mu.Unlock()
	if ok {
		return w, true, nil
	}

	var resp struct {
		JettonWallets []struct {
			Address string `json:"address"`
			Owner   string `json:"owner"`
			Jetton  string `json:"jetton"`
		} `json:"jetton_wallets"`
	}
	if err := c.get(ctx, "/jetton/wallets", map[string]string{
		"owner_address": owner.String(),
		// NOTE the parameter name. This endpoint spells it `jetton_address`
		// while /jetton/transfers spells the same thing `jetton_master`, and
		// passing the wrong one here is ignored in silence rather than
		// rejected. The verification below is what makes that survivable.
		"jetton_address": c.master.String(),
		"limit":          "100",
	}, &resp); err != nil {
		return Address{}, false, err
	}

	var found []Address
	for _, jw := range resp.JettonWallets {
		gotOwner, err := ParseAddress(jw.Owner)
		if err != nil || gotOwner != owner {
			continue
		}
		gotJetton, err := ParseAddress(jw.Jetton)
		if err != nil || gotJetton != c.master {
			continue
		}
		addr, err := ParseAddress(jw.Address)
		if err != nil {
			return Address{}, false, fmt.Errorf("tonrpc: jetton wallet %q for owner %s is not an address: %w", jw.Address, owner, err)
		}
		found = append(found, addr)
	}
	switch len(found) {
	case 0:
		return Address{}, false, nil
	case 1:
		c.mu.Lock()
		c.wallets[owner] = found[0]
		c.mu.Unlock()
		return found[0], true, nil
	default:
		// One owner has exactly one wallet per master on chain. Two means the
		// index is answering about something other than what we asked, and
		// picking one would be picking which account to credit from.
		return Address{}, false, fmt.Errorf("tonrpc: index reports %d jetton wallets for owner %s and master %s; there can be only one", len(found), owner, c.master)
	}
}

// --- the receiving wallet's transactions ---

type transaction struct {
	Account      string `json:"account"`
	Hash         string `json:"hash"`
	Lt           string `json:"lt"`
	McBlockSeqno uint64 `json:"mc_block_seqno"`
	InMsg        *struct {
		Source         string `json:"source"`
		Destination    string `json:"destination"`
		Opcode         string `json:"opcode"`
		Bounced        bool   `json:"bounced"`
		MessageContent *struct {
			Decoded map[string]any `json:"decoded"`
		} `json:"message_content"`
	} `json:"in_msg"`
	Description struct {
		Aborted   bool `json:"aborted"`
		ComputePh *struct {
			Skipped  bool `json:"skipped"`
			Success  bool `json:"success"`
			ExitCode int  `json:"exit_code"`
		} `json:"compute_ph"`
		Action *struct {
			Success    bool `json:"success"`
			ResultCode int  `json:"result_code"`
		} `json:"action"`
	} `json:"description"`
}

// receipts walks ONE jetton wallet's history back through the window and
// returns the credits it received.
//
// The walk is newest-first and pages by `end_lt`, not by `offset`. That is not
// a preference: an account's transactions grow at the head, so an offset-paged
// walk SKIPS rows whenever a new transaction lands between two pages, and a
// skipped row is a missed deposit. Logical time is strictly increasing within
// one account, so `end_lt = lowest lt seen − 1` is a cursor that cannot skip
// and cannot repeat, whatever arrives while we page.
func (c *Client) receipts(ctx context.Context, wallet Address, owner string, fromSeqno, toSeqno uint64) ([]depositwatch.Transfer, error) {
	var out []depositwatch.Transfer
	endLt := uint64(0)

	for page := 0; ; page++ {
		if page >= maxTransactionPages {
			return nil, fmt.Errorf("tonrpc: jetton wallet %s has more than %d transactions in masterchain blocks %d..%d — refusing to scan a partial window",
				wallet, maxTransactionPages*transactionPageLimit, fromSeqno, toSeqno)
		}
		params := map[string]string{
			"account": wallet.String(),
			"sort":    "desc",
			"limit":   strconv.Itoa(transactionPageLimit),
		}
		if endLt > 0 {
			params["end_lt"] = strconv.FormatUint(endLt, 10)
		}
		var resp struct {
			Transactions []transaction `json:"transactions"`
		}
		if err := c.get(ctx, "/transactions", params, &resp); err != nil {
			return nil, err
		}
		if len(resp.Transactions) == 0 {
			return out, nil
		}

		minLt := uint64(0)
		for i := range resp.Transactions {
			tx := &resp.Transactions[i]
			lt, err := strconv.ParseUint(strings.TrimSpace(tx.Lt), 10, 64)
			if err != nil {
				return nil, fmt.Errorf("tonrpc: transaction on %s has unreadable logical time %q: %w", wallet, tx.Lt, err)
			}
			if minLt == 0 || lt < minLt {
				minLt = lt
			}
			// Verify the account filter was actually applied (see get).
			if got, err := ParseAddress(tx.Account); err != nil || got != wallet {
				return nil, fmt.Errorf("tonrpc: asked for transactions of %s and was answered about %s — the endpoint ignored the account filter", wallet, tx.Account)
			}
			switch {
			case tx.McBlockSeqno == 0:
				// Indexed, but not yet placed in a masterchain block: it is at
				// the very tip, newer than any head we could have read. Skip it
				// WITHOUT stopping the walk — the rows behind it are older and
				// still wanted — and let a later pass credit it.
				continue
			case tx.McBlockSeqno > toSeqno:
				continue // newer than the window; belongs to the next pass
			case tx.McBlockSeqno < fromSeqno:
				// Older than the window, and so is everything behind it: within
				// one account, masterchain position rises with logical time.
				return out, nil
			}
			t, ok, err := receipt(tx, wallet, owner)
			if err != nil {
				return nil, err
			}
			if ok {
				out = append(out, t)
			}
		}

		if len(resp.Transactions) < transactionPageLimit || minLt <= 1 {
			return out, nil // the account's whole history is behind us
		}
		endLt = minLt - 1
	}
}

// receipt turns ONE transaction on our jetton wallet into the credit it
// produced, or reports that it produced none.
//
// The amount is taken from the INBOUND internal_transfer message's decoded
// body, and it is trustworthy for a reason that is worth stating: the jetton
// wallet contract itself validates the sender before crediting — a standard
// wallet accepts internal_transfer only from the jetton master or from the
// sibling wallet the transfer claims to come from — so a forged credit message
// makes the transaction FAIL on chain. The success check below is therefore not
// bookkeeping; it is what carries that consensus-enforced validation into this
// process. A transaction that aborted, whose compute phase failed or was
// skipped, or whose action phase failed, moved nothing.
func receipt(tx *transaction, wallet Address, owner string) (depositwatch.Transfer, bool, error) {
	var zero depositwatch.Transfer
	if tx.InMsg == nil || !strings.EqualFold(tx.InMsg.Opcode, opInternalTransfer) {
		return zero, false, nil // not a jetton credit
	}
	if tx.InMsg.Bounced {
		return zero, false, nil // a bounce of something we sent, not a deposit
	}
	if got, err := ParseAddress(tx.InMsg.Destination); err != nil || got != wallet {
		return zero, false, fmt.Errorf("tonrpc: transaction on %s carries an inbound message addressed to %q", wallet, tx.InMsg.Destination)
	}
	d := tx.Description
	if d.Aborted ||
		d.ComputePh == nil || d.ComputePh.Skipped || !d.ComputePh.Success || d.ComputePh.ExitCode != 0 ||
		(d.Action != nil && (!d.Action.Success || d.Action.ResultCode != 0)) {
		return zero, false, nil // failed on chain; no tokens moved
	}
	if tx.InMsg.MessageContent == nil || tx.InMsg.MessageContent.Decoded == nil {
		return zero, false, fmt.Errorf("tonrpc: transaction on %s carries an internal_transfer whose body the index did not decode — the amount is unknown", wallet)
	}
	body := tx.InMsg.MessageContent.Decoded
	if t, _ := body["@type"].(string); t != decodedInternalTransfer {
		return zero, false, fmt.Errorf("tonrpc: transaction on %s has opcode %s but a body decoded as %q", wallet, opInternalTransfer, t)
	}
	amount, ok := body["amount"].(map[string]any)
	if !ok {
		return zero, false, fmt.Errorf("tonrpc: transaction on %s carries an internal_transfer with no decoded amount", wallet)
	}
	raw, _ := amount["value"].(string)
	units, ok := new(big.Int).SetString(strings.TrimSpace(raw), 10)
	if !ok || units.Sign() < 0 {
		return zero, false, fmt.Errorf("tonrpc: transaction on %s carries amount %q, which is not a jetton base-unit amount", wallet, raw)
	}
	if units.Sign() == 0 {
		return zero, false, nil
	}
	hash, err := decodeHash(tx.Hash)
	if err != nil {
		return zero, false, fmt.Errorf("tonrpc: transaction on %s: %w", wallet, err)
	}
	return depositwatch.Transfer{
		To:     owner,
		Units:  units,
		TxHash: hash,
		// A TON transaction has AT MOST ONE inbound message — `in_msg` is a
		// single field on the transaction, not a list — so one transaction on a
		// jetton wallet is exactly one credit to it. The hash alone therefore
		// names this event, and 0 is the honest "there is only one" rather than
		// a placeholder. Two transfers into the same wallet are two
		// transactions, with two hashes and two logical times.
		EventIndex: 0,
		Block:      tx.McBlockSeqno,
	}, true, nil
}

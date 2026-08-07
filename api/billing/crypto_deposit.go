package billing

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/billing/depositledger"
	"github.com/hanzoai/commerce/billing/depositwatch"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/cryptopaymentintent"
	"github.com/hanzoai/commerce/payment/processor"
	jsonhttp "github.com/hanzoai/commerce/util/json/http"
)

// The native crypto top-up rail: a per-payer MPC custody deposit address.
//
// Address generation is the MPC signer's (processor.MPC — Hanzo KMS control
// plane + MPC signing backend); commerce holds NO key material and can only
// REQUEST an address, exactly like the HUSD treasury split. Crediting the
// balance is the chain watcher's job on real confirmations — nothing here
// mints, so a claimed deposit is worth nothing until the chain says otherwise.
//
// The supported chain/token set is served live from the processor
// (GET /v1/billing/crypto/options), so when the MPC service grows a chain
// (TON, XRP, …) it appears on every pay surface with no client change.

// cryptoDepositRequest is the wire body. There is deliberately NO subject
// field: the credited payer is the caller's own billing identity, so a body
// value cannot steer where a deposit lands.
type cryptoDepositRequest struct {
	Chain       string `json:"chain,omitempty"` // default "ethereum"
	Token       string `json:"token,omitempty"` // default "usdc"
	AmountCents int64  `json:"amountCents,omitempty"`
}

type cryptoDepositResponse struct {
	ID             string `json:"id"`
	Status         string `json:"status"`
	Chain          string `json:"chain"`
	Token          string `json:"token"`
	DepositAddress string `json:"depositAddress"`

	// AddressTag is the routing tag the payer MUST send with the payment on a
	// chain where the address is shared (XRPL's destination tag). Absent on
	// every chain that mints one address per payer.
	//
	// omitempty is safe and load-bearing here: the tag is rendered decimal, so
	// the first tag ever issued is the string "0", which is not empty and is
	// therefore sent. A payment to a pooled address with no tag names nobody
	// and is credited to nobody, so dropping "0" on the wire would strand the
	// deposit of whoever holds it.
	AddressTag string `json:"addressTag,omitempty"`

	ExpiresAt string `json:"expiresAt,omitempty"`
}

// cryptoProcessor resolves the ONE crypto custody processor (default MPC,
// override via COMMERCE_DEFAULT_CRYPTO_PROCESSOR upstream in the registry).
// Availability is probed with a short deadline so an unconfigured or wedged
// MPC endpoint answers 503 fast instead of hanging the request.
func cryptoProcessor(ctx context.Context) (processor.CryptoProcessor, error) {
	p, err := processor.Get(processor.MPC)
	if err != nil {
		return nil, err
	}
	cp, ok := p.(processor.CryptoProcessor)
	if !ok {
		return nil, fmt.Errorf("processor %s does not support crypto operations", processor.MPC)
	}
	probe, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if !cp.IsAvailable(probe) {
		return nil, fmt.Errorf("crypto custody service unavailable")
	}
	return cp, nil
}

// GetCryptoOptions lists the assets the rail accepts — which is the set a
// deposit watcher can CREDIT, not the set the MPC signer can mint an address
// for. Those are different questions and only the second one was ever asked
// here.
//
// The processor answers "can I derive an address on this chain?", and it says
// yes to nine of them. Nothing followed a single one of those addresses to the
// customer's balance, so the picker walked buyers toward assets the system
// could receive and never credit — the same defect that stopped this rail, one
// endpoint upstream of it.
//
// Deriving the list from the watcher's configured assets makes that
// unrepresentable: an asset appears here only because something is watching it,
// and it stops appearing the moment that stops being true. An unconfigured
// watcher therefore offers nothing rather than everything, which is the safe
// direction for a list whose entries invite people to send money.
//
//	GET /v1/billing/crypto/options
func GetCryptoOptions(c *zip.Ctx) error {
	// The custody signer must still be reachable — an asset nobody can mint an
	// address for is no more useful than one nobody can credit.
	cp, err := cryptoProcessor(c.Context())
	if err != nil {
		return jsonhttp.Fail(c, 503, "Crypto deposits not configured", err)
	}

	chains, tokens := offeredFrom(watchedAssets(), cp.SupportedChains())
	return c.JSON(200, map[string]any{"chains": chains, "tokens": tokens})
}

// watchedAssets is the set of assets something is actually watching — the ONE
// question both the picker and the mint path ask, so they cannot disagree about
// what is on offer. An unconfigured or disabled watcher yields none, which is
// what makes "offered" and "creditable" the same set.
func watchedAssets() []depositwatch.Asset {
	if w := depositledger.Default(); w.Enabled() {
		return w.Assets()
	}
	return nil
}

// pooledAddressFor returns the ONE custody account (chain, token) is deposited
// to, for a chain whose deposits are pooled. Empty when nothing watches that
// asset — which is the only reason it can be empty, since AssetsFromEnv refuses
// to build a pooled asset with no configured account.
//
// It is looked up per (chain, TOKEN), not per chain, and that is the check it
// exists for: handing out the pooled address for a token nothing watches would
// invite a deposit that no scan will ever see. The address itself is per-chain
// — one account holds every currency sent to it — so every token on a pooled
// chain answers the same string.
func pooledAddressFor(assets []depositwatch.Asset, chain, token string) string {
	for _, a := range assets {
		if strings.EqualFold(a.Chain, chain) && strings.EqualFold(a.Token, token) {
			return a.PooledAddress
		}
	}
	return ""
}

// offeredFrom projects the watched assets onto the two lists the picker renders,
// keeping only the chains an address can actually be MINTED on.
//
// Both halves are required, and each one alone has already shipped a dead end.
// This endpoint originally answered from the custody signer — "can I derive an
// address here?" — and so offered nine chains the watcher credited none of.
// Answering from the watcher alone inverts the same defect: Solana is creditable
// the moment it is configured, but the custody fleet derives no Ed25519 key and
// `CreateCryptoDeposit` refuses the chain, so the picker would walk a buyer to a
// 400 after they had chosen an amount. An asset is offered when it can be
// received AND credited, which is the only combination that ends with a
// customer's balance going up.
//
// It is a pure function of its two inputs and not a method on the watcher
// because that is what makes the rule testable: NO assets must yield NO offer,
// and the only way to be sure of that is to be able to ask it directly. Reading
// the process-wide watcher inside the handler would leave the question
// answerable only by whatever the environment happened to configure.
//
// Both lists are deduplicated and sorted, so the picker's order is a property of
// the inputs rather than of map iteration.
func offeredFrom(assets []depositwatch.Asset, mintable []string) (chains, tokens []string) {
	canMint := make(map[string]bool, len(mintable))
	for _, ch := range mintable {
		canMint[strings.ToLower(strings.TrimSpace(ch))] = true
	}
	cset, tset := map[string]bool{}, map[string]bool{}
	for _, a := range assets {
		chain := strings.ToLower(a.Chain)
		// The custody signer is asked only about chains it would actually have
		// to mint on. A POOLED chain's address is CONFIGURED, not derived, so
		// the signer's opinion of it is not just unnecessary but wrong: this
		// signer answers "no" for XRPL (it derives no XRPL key, and its chain
		// switch falls through to the EVM address), and deferring to that would
		// hide a chain the rail can both receive and credit. What makes a
		// pooled chain offerable is that its account is configured — and
		// AssetsFromEnv refuses to build the asset otherwise, so being watched
		// already IS that.
		if !a.Pooled() && !canMint[chain] {
			continue
		}
		cset[chain] = true
		// A token is named only because some OFFERED chain carries it. Listing
		// it from an unofferable chain would put it in the picker with nowhere
		// to send it.
		tset[strings.ToLower(a.Token)] = true
	}
	return sortedKeys(cset), sortedKeys(tset)
}

// sortedKeys returns the set's members in order, and NEVER nil — a nil slice
// marshals to `null`, and a picker handed `null` where it expects a list is a
// client-side crash rather than an empty menu.
func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// cryptoDepositsCanBeCredited gates the rail on the ONE property that makes
// taking crypto legitimate: that a deposit can reach the customer's balance.
// It is false because no component advances a CryptoPaymentIntent past Pending
// and no watcher observes DepositAddress — so a deposit would be money received
// and never credited.
//
// It is a constant rather than config on purpose: an operator must not be able
// to turn this on from the outside. The thing that makes it safe is code that
// does not exist yet, so only code may flip it.
const cryptoDepositsCanBeCredited = false

// openIntentFor returns the payer's live deposit intent for this asset, if
// there is one to reuse.
//
// One payer, one live destination per asset. Reuse is what stops a refreshed
// page from spraying MPC keygens and stranding funds across addresses — and on
// a POOLED chain it is what stops it spraying destination TAGS, which matters
// more: a tag is drawn from a finite space that is never reclaimed, and every
// intent ever minted stays watched forever (depositledger.Watched filters on
// the asset and deliberately not on status). Reuse allocates nothing, so the
// second request through this door costs the sequence nothing.
//
// A reused intent is returned whole rather than as an address, so its tag
// travels with it and the two halves of its identity cannot come from different
// places.
//
// Three conditions, each of which has to hold:
//
//	found + PENDING   a settled or failed intent is history, not a destination
//	not expired       expiry governs whether we hand the address out AGAIN; it
//	                  never governs whether we honour what arrives at it
//	has an address    an intent that never got one is nothing to reuse
//
// Get reports a miss as (false, nil), so the caller must check found and not
// merely err.
func openIntentFor(db *datastore.Datastore, payer, chain, token string) (*cryptopaymentintent.CryptoPaymentIntent, bool) {
	existing := cryptopaymentintent.New(db)
	found, err := existing.Query().
		Filter("CustomerRef=", payer).
		Filter("Chain=", chain).
		Filter("Token=", token).
		Filter("Status=", string(cryptopaymentintent.Pending)).
		Get()
	if err != nil || !found || existing.IsExpired() || existing.DepositAddress == "" {
		return nil, false
	}
	return existing, true
}

// destinationIsComplete reports whether an intent records a destination a payer
// can actually be sent to — BOTH halves of it, on a chain that has two.
//
// It reads the chain off the INTENT rather than off the request, because the
// stored row is the thing being handed back and it is the row that has to be
// coherent. On every per-payer chain it is trivially true, so nothing about
// those paths changes: the address is the whole destination there.
//
// It exists for one state that this handler cannot produce — a pooled intent
// with no tag, since the tag is allocated and written in the same record as the
// address — but which a hand-edited row, a restore, or an older binary could.
// Such a row is not merely incomplete, it is a trap: the address is real and
// ours, so a payment to it ARRIVES and is credited to nobody.
func destinationIsComplete(in *cryptopaymentintent.CryptoPaymentIntent) bool {
	return !depositwatch.Pooled(string(in.Chain)) || in.AddressTag != ""
}

// CreateCryptoDeposit answers a custody deposit address for the caller. An
// open PENDING intent for the same (payer, chain, token) is reused — one
// payer, one live address per asset — so refreshing the page cannot spray
// MPC keygens or strand funds across addresses.
//
//	POST /v1/billing/crypto/deposit   { chain?, token?, amountCents? }
func CreateCryptoDeposit(c *zip.Ctx) error {
	// STOPPED: handing out an address here takes money we cannot credit.
	//
	// The rail mints a real custody address and records an intent as Pending —
	// and nothing in this codebase ever moves it off Pending. MarkConfirming and
	// MarkSucceeded (models/cryptopaymentintent) have NO production callers; the
	// only writer of Status is the Pending set below. There is no chain watcher:
	// husdindex scans one ERC-20 on one chain against seed-derived treasury
	// addresses, never DepositAddress, and it is not scheduled. The pay SPA's
	// "I sent the crypto" is a GET that re-reads the same Pending row. The
	// ledger primitives that could credit (POST /billing/credit, /billing/deposit)
	// are mint-gated and unreachable from here.
	//
	// Worse, GenerateAddress DISCARDS the keygen response's wallet_id and returns
	// only the address string, so we do not even retain a handle to the MPC wallet
	// holding the coins — recovering a stranded deposit means reconciling against
	// the node's own wallet records.
	//
	// Three comments in this tree assert "the chain watcher credits on real
	// confirmations". No such component exists. That sentence is why this shipped.
	//
	// So the rail refuses to take money it cannot credit. This is deliberately at
	// the TOP of the handler, before any keygen: an address that is never minted
	// is an address nobody can send to. Reads (GetCryptoDeposit, GetCryptoOptions)
	// are untouched so an existing intent can still be inspected.
	//
	// TO LIFT THIS: a per-chain deposit watcher that observes DepositAddress,
	// advances the intent, and credits through the ledger exactly once — plus
	// persisting wallet_id so a deposit is recoverable. Flip the constant in the
	// same commit that makes the credit path real, never before.
	if !cryptoDepositsCanBeCredited {
		return jsonhttp.Fail(c, 503,
			"Crypto deposits are paused. Funds sent to a crypto address cannot be credited yet, so we will not issue one. Use a card, bank transfer or wire.", nil)
	}

	payer := userBillingKey(c)
	if payer == "" {
		return jsonhttp.Fail(c, 401, "Authentication required", nil)
	}
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	var req cryptoDepositRequest
	if len(c.Body()) > 0 {
		if err := json.Unmarshal(c.Body(), &req); err != nil {
			return jsonhttp.Fail(c, 400, "invalid request body", err)
		}
	}
	chain := strings.ToLower(strings.TrimSpace(req.Chain))
	if chain == "" {
		chain = "ethereum"
	}
	token := strings.ToLower(strings.TrimSpace(req.Token))
	if token == "" {
		token = "usdc"
	}

	// A POOLED chain does not mint anything, so it asks the custody signer
	// nothing. Its account is configured (depositwatch.Asset.PooledAddress) and
	// what distinguishes one payer's deposit from another's is a tag allocated
	// below — so probing the signer here would only add a way for a working
	// rail to refuse, and asking it whether it "supports xrpl" would answer no
	// and close a chain that works. Every per-payer chain keeps the path it
	// had, in the order it had it.
	pooled := depositwatch.Pooled(chain)
	var cp processor.CryptoProcessor
	if !pooled {
		var err error
		cp, err = cryptoProcessor(c.Context())
		if err != nil {
			return jsonhttp.Fail(c, 503, "Crypto deposits not configured", err)
		}
		if !supportedChain(cp, chain) {
			return jsonhttp.Fail(c, 400, fmt.Sprintf("unsupported chain %q", chain), nil)
		}
	}

	// Reuse the payer's open intent for this asset before minting a new one.
	if existing, ok := openIntentFor(db, payer, chain, token); ok {
		// A pooled destination missing its tag names NOBODY: the payment would
		// arrive at an address we own and be credited to no one (recorded as
		// unattributed, refundable only by hand). It cannot arise from this
		// handler — the tag is allocated and written in the same record as the
		// address — so a row like it is corruption, and the two tempting
		// repairs are both worse than refusing. Handing it over takes money we
		// cannot route; minting a replacement leaves the untagged row live and
		// sprays the tag space on every refresh.
		if !destinationIsComplete(existing) {
			log.Error("crypto deposit intent %s on %s has no destination tag; refusing to reuse it", existing.Id(), chain, c)
			return jsonhttp.Fail(c, 503, "Crypto deposits are temporarily unavailable — this deposit reference is incomplete. Contact support or choose another way to pay.", nil)
		}
		return c.JSON(200, toCryptoDepositResponse(existing))
	}

	// WHERE the payer sends, and WHAT names them when they arrive. This is the
	// only thing the two kinds of chain answer differently; everything after it
	// is one path, so a field can never be recorded on one kind of chain and
	// quietly forgotten on the other.
	var wallet processor.Wallet
	var tag string
	if pooled {
		// The account is configured, never invented — and never asked of the
		// custody signer, which would mint a fresh wallet per deposit and
		// strand a non-refundable reserve on each one. If nothing is
		// configured, nothing is handed out.
		addr := pooledAddressFor(watchedAssets(), chain, token)
		if addr == "" {
			return jsonhttp.Fail(c, 503, fmt.Sprintf("Crypto deposits on %s are not configured — no custody account is set for %s.", chain, strings.ToUpper(token)), nil)
		}
		// The tag is allocated BY THE DATABASE and never chosen here — see
		// depositledger.NextTag for what makes it unique across replicas, and
		// why a random tag with a uniqueness check would not be. Allocating one
		// and not using it costs nothing, since the sequence simply moves on;
		// issuing one twice halts the asset for every customer. So this refuses
		// rather than falls back.
		var err error
		if tag, err = depositledger.NextTag(c.Context(), chain); err != nil {
			log.Error("crypto deposit tag allocation failed for %q on %s: %v", payer, chain, err, c)
			return jsonhttp.Fail(c, 503, "Crypto deposits are temporarily unavailable — a deposit reference could not be issued. Try again shortly or choose another way to pay.", err)
		}
		// No wallet ID. The custody signer holds no per-deposit wallet here —
		// the pooled account IS the wallet, and it is the operator's rather
		// than this intent's. Empty is the honest answer, not a missing one.
		wallet = processor.Wallet{Address: addr}
	} else {
		var err error
		if wallet, err = cp.GenerateAddress(c.Context(), payer, chain); err != nil {
			log.Error("crypto deposit address generation failed for %q on %s: %v", payer, chain, err, c)
			// 503, NOT 502 — and the difference is what the customer reads.
			// Cloudflare REPLACES an origin 502 with its own "Bad gateway"
			// interstitial, so the JSON below never reaches the browser: measured on
			// pay.hanzo.ai, the deposit call returned a text/html CF error page while
			// the identical call to the origin returned this message. 503 passes
			// through untouched (the wire rail's own "not configured" 503 proves it),
			// which is also the honest code: the custody signer is unavailable, we
			// are not a broken gateway.
			return jsonhttp.Fail(c, 503, "Crypto deposits are temporarily unavailable — the custody service is not accepting new addresses. Try again shortly or choose another way to pay.", err)
		}
	}

	intent, err := recordIntent(db, payer, chain, token, req.AmountCents, wallet, tag)
	if err != nil {
		return jsonhttp.Fail(c, 500, "failed to record deposit intent", err)
	}

	return c.JSON(200, toCryptoDepositResponse(intent))
}

// recordIntent turns a resolved destination into the durable row the watcher
// will later match deposits against.
//
// It is a function and not four lines in the handler for a reason that is not
// tidiness: CreateCryptoDeposit is gated shut (cryptoDepositsCanBeCredited), so
// anything written inside it is UNTESTABLE — a mutant that recorded the pooled
// address and dropped the tag passed the entire suite, because no test can
// reach the statement. Money-carrying assignments do not belong behind a gate
// no test can open.
//
// It REFUSES to write an incomplete destination. That is the same invariant
// destinationIsComplete states on the way out, enforced here on the way IN, so
// a row that traps a payment cannot be created rather than merely being caught
// later: on a pooled chain the address is real and ours, so a payment to it
// arrives whether or not we recorded who it belongs to.
func recordIntent(db *datastore.Datastore, payer, chain, token string, amountCents int64, wallet processor.Wallet, tag string) (*cryptopaymentintent.CryptoPaymentIntent, error) {
	intent := cryptopaymentintent.New(db)
	intent.Amount = amountCents
	intent.Currency = "usd"
	intent.Chain = cryptopaymentintent.Chain(chain)
	intent.Token = token
	intent.DepositAddress = wallet.Address
	// The other half of the destination on a pooled chain, and empty on every
	// chain where the address is the whole answer. Written in the SAME record
	// as the address, because an intent holding one without the other names an
	// identity the watcher can never match.
	intent.AddressTag = tag
	// Recorded at mint time because this is the only moment the signer offers
	// it. Without it a deposit can be credited and never swept.
	intent.WalletID = wallet.ID
	intent.CustomerRef = payer
	intent.Status = cryptopaymentintent.Pending
	// A top-up address is a standing destination, not a checkout hold — give it
	// a day rather than the model's 30-minute checkout default.
	intent.ExpiresAt = time.Now().Add(24 * time.Hour)
	intent.Defaults()

	if intent.DepositAddress == "" {
		return nil, fmt.Errorf("refusing to record a %s/%s deposit intent with no address", chain, token)
	}
	if !destinationIsComplete(intent) {
		return nil, fmt.Errorf("refusing to record a %s/%s deposit intent with an address but no destination tag — a payment to it would arrive and be credited to nobody", chain, token)
	}
	if err := intent.Create(); err != nil {
		return nil, err
	}
	return intent, nil
}

// GetCryptoDeposit reports an intent's state (pending → confirming →
// succeeded), scoped to the caller: another payer's intent id answers 404.
//
//	GET /v1/billing/crypto/deposit/:id
func GetCryptoDeposit(c *zip.Ctx) error {
	payer := userBillingKey(c)
	if payer == "" {
		return jsonhttp.Fail(c, 401, "Authentication required", nil)
	}
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	intent := cryptopaymentintent.New(db)
	if err := intent.GetById(c.Param("id")); err != nil {
		return jsonhttp.Fail(c, 404, "deposit not found", err)
	}
	if intent.CustomerRef != payer {
		return jsonhttp.Fail(c, 404, "deposit not found", nil)
	}
	return c.JSON(200, toCryptoDepositResponse(intent))
}

func toCryptoDepositResponse(i *cryptopaymentintent.CryptoPaymentIntent) cryptoDepositResponse {
	return cryptoDepositResponse{
		ID:             i.Id(),
		Status:         string(i.Status),
		Chain:          string(i.Chain),
		Token:          i.Token,
		DepositAddress: i.DepositAddress,
		AddressTag:     i.AddressTag,
		ExpiresAt:      i.ExpiresAt.UTC().Format(time.RFC3339),
	}
}

func supportedChain(cp processor.CryptoProcessor, chain string) bool {
	for _, s := range cp.SupportedChains() {
		if s == chain {
			return true
		}
	}
	return false
}

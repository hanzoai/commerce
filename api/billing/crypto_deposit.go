package billing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/billing/depositledger"
	"github.com/hanzoai/commerce/billing/depositwatch"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/cryptopaymentintent"
	"github.com/hanzoai/commerce/models/organization"
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

// CryptoDeposit is a payer's destination and what has become of it — where to
// send, what names them when it arrives, and how far the chain has got. Both
// endpoints hand back this same value, so a deposit reads identically over HTTP and
// over the internal plane.
type CryptoDeposit struct {
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

// CryptoOptions is the menu of assets the rail accepts: the chains a payer may
// send on and the tokens they may send, both already deduplicated and ordered.
// Neither list is ever nil — a picker handed `null` where it expects an array is
// a client-side crash rather than an empty menu.
type CryptoOptions struct {
	Chains []string `json:"chains"`
	Tokens []string `json:"tokens"`
}

// depositValidationError marks a request that named an asset this rail cannot
// mint on — the caller's own doing, and fixable by asking for a different one.
// The subscription core states its client-side failures the same way
// (subValidationError).
type depositValidationError struct{ msg string }

func (e depositValidationError) Error() string { return e.msg }

// depositRefusal is the rail itself being shut for an asset: nothing is watching
// it, no custody account is configured, the signer will not mint, a reference
// could not be issued. msg is the sentence the PAYER reads and names another way
// to pay; cause carries the operational detail for the log, and is nil where
// there is no underlying failure to report.
type depositRefusal struct {
	msg   string
	cause error
}

func (e depositRefusal) Error() string { return e.msg }
func (e depositRefusal) Unwrap() error { return e.cause }

// errDepositNotFound is the ONE answer for both an id that names nothing and an
// id that names another payer's intent. They are deliberately the same value:
// distinguishing them would let a caller confirm a foreign intent exists, which
// is the whole reason the read is payer-scoped.
var errDepositNotFound = errors.New("crypto deposit: no such deposit for this payer")

// IsDepositUnsupported reports whether the rail turned down the ASSET that was
// asked for — a chain the custody signer cannot mint an address on. Asking for a
// different one is the fix, which is why this is a separate question from
// IsDepositRefused, and why the endpoint answers it with a 400.
func IsDepositUnsupported(err error) bool {
	var e depositValidationError
	return errors.As(err, &e)
}

// IsDepositRefused reports whether the rail is SHUT for that asset — nothing is
// watching it, or nothing can mint on it right now. Nothing sent while this
// holds could be credited, so asking differently does not help and only time or
// an operator does; the endpoint answers it with a 503 and Error() is the sentence
// to show the payer.
func IsDepositRefused(err error) bool {
	var e depositRefusal
	return errors.As(err, &e)
}

// IsDepositNotFound reports the miss. An unknown id and another payer's id are
// one answer here, on purpose: a caller that could tell them apart could confirm
// a foreign deposit exists.
func IsDepositNotFound(err error) bool { return errors.Is(err, errDepositNotFound) }

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
// It takes a context and nothing else because that is the whole of what the
// question depends on: the offer is a property of this process — which assets
// its watcher reads, which chains its custody signer will mint on — and not of
// the org asking. A peer that offers the same picker over the internal plane
// asks here rather than keeping a list of its own, since a second list is how a
// buyer comes to be shown an asset the mint path refuses.
func GetCryptoOptions(ctx context.Context) (CryptoOptions, error) {
	// The custody signer must still be reachable — an asset nobody can mint an
	// address for is no more useful than one nobody can credit.
	cp, err := cryptoProcessor(ctx)
	if err != nil {
		return CryptoOptions{}, err
	}
	chains, tokens := offeredFrom(watchedAssets(), cp.SupportedChains())
	return CryptoOptions{Chains: chains, Tokens: tokens}, nil
}

// GetBillingCryptoOptions is the picker's endpoint for that menu.
//
//	GET /v1/billing/crypto/options
func GetBillingCryptoOptions(c *zip.Ctx) error {
	out, err := GetCryptoOptions(c.Context())
	if err != nil {
		return jsonhttp.Fail(c, 503, "Crypto deposits not configured", err)
	}
	return c.JSON(200, out)
}

// watchedAssets is the set of assets something is actually watching — the ONE
// question both the picker and the mint path ask, so they cannot disagree about
// what is on offer. An unconfigured, disabled or STOPPED watcher yields none,
// which is what makes "offered" and "creditable" the same set.
//
// It asks Running, not Enabled, and the difference is the whole safety property.
// Enabled means "an asset was configured"; Running means "something is reading
// the chain for it". Only the second one implies a deposit reaches a balance.
// Gating on the first would re-create the original bug in a new place: a process
// that constructed the watcher and never started its schedule would hand out
// real custody addresses that nothing on earth will ever look at — which is
// precisely the shape of the failure this rail exists to end, and it would look
// healthy from outside.
//
// That makes the gate self-enforcing rather than ceremonial. Bootstrap no longer
// starts the schedule (it builds the app; it does not run it), so the start now
// lives in the two serving entry points — and a third one added later that
// forgets cannot take money, it can only refuse. Fail closed, by construction,
// with no second thing to remember.
func watchedAssets() []depositwatch.Asset {
	if w := depositledger.Default(); w.Running() {
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

// creditable reports whether a deposit of this asset can actually reach the
// customer's balance — which is the ONE property that makes taking crypto
// legitimate, and the question a blanket flag could never answer.
//
// This replaces `const cryptoDepositsCanBeCredited = false`. That constant was
// right for what it knew: when it was written NOTHING advanced an intent past
// Pending and no watcher observed DepositAddress, so no asset was creditable and
// one `false` covered every case. All four of its stated lift conditions are now
// met — a watcher observes the address, advances the intent, credits exactly
// once through the ledger, and wallet_id persists so a credit can also be swept.
//
// But flipping that constant to `true` would have reintroduced the ORIGINAL BUG
// for every asset the watcher is not configured for. "Crediting exists" is not
// "crediting exists FOR THIS CHAIN": the watcher is configured per asset from
// CRYPTO_DEPOSIT_*, so an unconfigured chain still mints a real custody address
// that nothing on earth will ever look at. A global boolean cannot express a
// per-asset fact, and the gap between them is exactly where money was lost.
//
// So the gate is now the invariant itself, asked per request: this asset is
// offered only if something is watching it. An operator cannot turn it on from
// outside — configuring the watcher IS turning it on, and that is the same act
// rather than a second one that has to be remembered. Nothing is watched by
// default, so the rail is closed until an asset is deliberately armed.
// It reads watchedAssets(), the SAME accessor GetCryptoOptions projects the
// picker from, so "offered" and "mintable" cannot drift apart: a buyer can only
// be shown an asset this will hand out an address for, and vice versa.
func creditable(assets []depositwatch.Asset, chain, token string) bool {
	for _, a := range assets {
		if strings.EqualFold(a.Chain, chain) && strings.EqualFold(a.Token, token) {
			return true
		}
	}
	return false
}

// openIntentFor returns the payer's live deposit intent for this asset, if
// there is one to reuse.
//
// One payer, one live destination per asset. Reuse is what stops a refreshed
// page from spraying MPC keygens and stranding funds across addresses — and on
// a POOLED chain it is what stops it spraying destination TAGS, which matters
// more: a tag is drawn from a finite space that is never reclaimed, and every
// intent ever minted stays watched forever (depositledger.Watched filters on
// the asset and deliberately not on status). Reuse allocates nothing, so the
// second request through this endpoint costs the sequence nothing.
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

// CreateCryptoDeposit answers where a payer sends and what names them when it
// arrives — the MINT, with no HTTP in it.
//
// It takes values rather than a request because the peer that offers this rail
// over the internal plane holds no datastore, and two implementations of "where
// does this payer send, and may they" is how one endpoint comes to hand out an
// address the other refuses. The asset defaults here rather than at the endpoint for
// the same reason: an unnamed chain means ethereum wherever it is asked from.
//
// The payer is the CALLER's own billing identity, resolved at the endpoint and
// passed in. A deposit lands where the credential says and nowhere else.
//
// An open PENDING intent for the same (payer, chain, token) is reused — one
// payer, one live address per asset — so refreshing the page cannot spray MPC
// keygens, strand funds across addresses, or consume the finite tag space.
//
// The two refusals are separate types because they mean opposite things to
// whoever asked: IsDepositUnsupported says ask for another asset,
// IsDepositRefused says nothing sent now could be credited. Anything else is a
// write that failed.
func CreateCryptoDeposit(ctx context.Context, org *organization.Organization, payer, chain, token string, amountCents int64) (CryptoDeposit, error) {
	if org == nil {
		return CryptoDeposit{}, errors.New("crypto deposit: no organization")
	}
	if payer == "" {
		return CryptoDeposit{}, errors.New("crypto deposit: no payer")
	}
	db := datastore.New(org.Namespaced(ctx))

	chain = strings.ToLower(strings.TrimSpace(chain))
	if chain == "" {
		chain = "ethereum"
	}
	token = strings.ToLower(strings.TrimSpace(token))
	if token == "" {
		token = "usdc"
	}

	// THE GATE, and it is the whole reason this rail may take money at all: an
	// address is handed out only for an asset something is watching. Asked before
	// any keygen, because an address that is never minted is one nobody can send
	// to — and asked from the SAME accessor the picker projects from, so a buyer
	// can never be offered an asset this would refuse.
	//
	// Nothing is watched until CRYPTO_DEPOSIT_* names it, so the rail is closed by
	// default and arming an asset is one deliberate act rather than two that must
	// be done in the right order.
	watched := watchedAssets()
	if !creditable(watched, chain, token) {
		return CryptoDeposit{}, depositRefusal{msg: fmt.Sprintf(
			"%s on %s is not accepted yet — nothing is watching that address, so a deposit could not be credited. Use a card, bank transfer or wire.",
			strings.ToUpper(token), chain)}
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
		cp, err = cryptoProcessor(ctx)
		if err != nil {
			return CryptoDeposit{}, depositRefusal{msg: "Crypto deposits not configured", cause: err}
		}
		if !supportedChain(cp, chain) {
			return CryptoDeposit{}, depositValidationError{fmt.Sprintf("unsupported chain %q", chain)}
		}
	}

	// Reuse the payer's open intent for this asset before minting a new one.
	if existing, ok := openIntentFor(db, payer, chain, token); ok {
		// A pooled destination missing its tag names NOBODY: the payment would
		// arrive at an address we own and be credited to no one (recorded as
		// unattributed, refundable only by hand). It cannot arise from this
		// path — the tag is allocated and written in the same record as the
		// address — so a row like it is corruption, and the two tempting
		// repairs are both worse than refusing. Handing it over takes money we
		// cannot route; minting a replacement leaves the untagged row live and
		// sprays the tag space on every refresh.
		if !destinationIsComplete(existing) {
			return CryptoDeposit{}, depositRefusal{
				msg:   "Crypto deposits are temporarily unavailable — this deposit reference is incomplete. Contact support or choose another way to pay.",
				cause: fmt.Errorf("crypto deposit intent %s on %s has no destination tag; refusing to reuse it", existing.Id(), chain),
			}
		}
		return toCryptoDepositResponse(existing), nil
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
		addr := pooledAddressFor(watched, chain, token)
		if addr == "" {
			return CryptoDeposit{}, depositRefusal{msg: fmt.Sprintf(
				"Crypto deposits on %s are not configured — no custody account is set for %s.", chain, strings.ToUpper(token))}
		}
		// The tag is allocated BY THE DATABASE and never chosen here — see
		// depositledger.NextTag for what makes it unique across replicas, and
		// why a random tag with a uniqueness check would not be. Allocating one
		// and not using it costs nothing, since the sequence simply moves on;
		// issuing one twice halts the asset for every customer. So this refuses
		// rather than falls back.
		var err error
		if tag, err = depositledger.NextTag(ctx, chain); err != nil {
			return CryptoDeposit{}, depositRefusal{
				msg:   "Crypto deposits are temporarily unavailable — a deposit reference could not be issued. Try again shortly or choose another way to pay.",
				cause: fmt.Errorf("crypto deposit tag allocation failed for %q on %s: %w", payer, chain, err),
			}
		}
		// No wallet ID. The custody signer holds no per-deposit wallet here —
		// the pooled account IS the wallet, and it is the operator's rather
		// than this intent's. Empty is the honest answer, not a missing one.
		wallet = processor.Wallet{Address: addr}
	} else {
		var err error
		if wallet, err = cp.GenerateAddress(ctx, payer, chain); err != nil {
			return CryptoDeposit{}, depositRefusal{
				msg:   "Crypto deposits are temporarily unavailable — the custody service is not accepting new addresses. Try again shortly or choose another way to pay.",
				cause: fmt.Errorf("crypto deposit address generation failed for %q on %s: %w", payer, chain, err),
			}
		}
	}

	intent, err := recordIntent(db, payer, chain, token, amountCents, wallet, tag)
	if err != nil {
		return CryptoDeposit{}, err
	}

	return toCryptoDepositResponse(intent), nil
}

// CreateBillingCryptoDeposit is the payer's endpoint for the crypto rail. It
// resolves who is asking, reads the asset off the body, and maps a refusal to
// the code it has always mapped to.
//
//	POST /v1/billing/crypto/deposit   { chain?, token?, amountCents? }
func CreateBillingCryptoDeposit(c *zip.Ctx) error {
	payer := userBillingKey(c)
	if payer == "" {
		return jsonhttp.Fail(c, 401, "Authentication required", nil)
	}
	org := middleware.GetOrganization(c)

	var req cryptoDepositRequest
	if len(c.Body()) > 0 {
		if err := json.Unmarshal(c.Body(), &req); err != nil {
			return jsonhttp.Fail(c, 400, "invalid request body", err)
		}
	}

	out, err := CreateCryptoDeposit(c.Context(), org, payer, req.Chain, req.Token, req.AmountCents)
	switch {
	case IsDepositUnsupported(err):
		return jsonhttp.Fail(c, 400, err.Error(), nil)
	case IsDepositRefused(err):
		// 503, NOT 502 — and the difference is what the customer reads.
		// Cloudflare REPLACES an origin 502 with its own "Bad gateway"
		// interstitial, so the JSON never reaches the browser: measured on
		// pay.hanzo.ai, the deposit call returned a text/html CF error page while
		// the identical call to the origin returned this message. 503 passes
		// through untouched (the wire rail's own "not configured" 503 proves it),
		// which is also the honest code: the rail is unavailable, we are not a
		// broken gateway.
		return jsonhttp.Fail(c, 503, err.Error(), errors.Unwrap(err))
	case err != nil:
		return jsonhttp.Fail(c, 500, "failed to record deposit intent", err)
	}
	return c.JSON(200, out)
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
// succeeded) — the READ, with no HTTP in it.
//
// It takes the payer as a value because the read is SCOPED to them and the scope
// is the point: the same lookup answered over the internal plane must refuse a
// foreign id there too, and a second copy of "is this yours" is a copy that can
// be written without the check. An id that names nothing and an id that names
// somebody else return the SAME error, so neither endpoint can be made to confirm
// that a foreign intent exists.
func GetCryptoDeposit(ctx context.Context, org *organization.Organization, payer, id string) (CryptoDeposit, error) {
	if org == nil {
		return CryptoDeposit{}, errors.New("crypto deposit: no organization")
	}
	if payer == "" {
		return CryptoDeposit{}, errors.New("crypto deposit: no payer")
	}
	intent := cryptopaymentintent.New(datastore.New(org.Namespaced(ctx)))
	if err := intent.GetById(id); err != nil {
		return CryptoDeposit{}, errDepositNotFound
	}
	if intent.CustomerRef != payer {
		return CryptoDeposit{}, errDepositNotFound
	}
	return toCryptoDepositResponse(intent), nil
}

// GetBillingCryptoDeposit is the payer's endpoint for their own deposit.
//
//	GET /v1/billing/crypto/deposit/:id
func GetBillingCryptoDeposit(c *zip.Ctx) error {
	payer := userBillingKey(c)
	if payer == "" {
		return jsonhttp.Fail(c, 401, "Authentication required", nil)
	}
	out, err := GetCryptoDeposit(c.Context(), middleware.GetOrganization(c), payer, c.Param("id"))
	if err != nil {
		return jsonhttp.Fail(c, 404, "deposit not found", err)
	}
	return c.JSON(200, out)
}

func toCryptoDepositResponse(i *cryptopaymentintent.CryptoPaymentIntent) CryptoDeposit {
	return CryptoDeposit{
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

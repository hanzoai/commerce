package billing

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/paymentmethod"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/util/nscontext"
)

// cardVault is the subset of processor.CustomerProcessor needed to vault a card
// on file: create/reuse a customer and attach/detach a card. Stripe and Square
// satisfy it; the narrow interface keeps the billing handlers decoupled from
// either SDK.
type cardVault interface {
	processor.PaymentProcessor

	// CreateCustomer creates a customer profile and returns its id.
	CreateCustomer(ctx context.Context, email, name string, metadata map[string]interface{}) (string, error)

	// AddPaymentMethod attaches a single-use card nonce to a customer and
	// returns the durable card-on-file id.
	AddPaymentMethod(ctx context.Context, customerID, token string) (string, error)

	// RemovePaymentMethod detaches a card-on-file from a customer.
	RemovePaymentMethod(ctx context.Context, customerID, paymentMethodID string) error
}

// vaulter is the richer vault face: the same attach, returning the card as the
// processor reports it (brand/last4/expiry/fingerprint). Optional — detected by
// assertion so a processor (or an old fake) that only has AddPaymentMethod
// still vaults, just without card facts.
type vaulter interface {
	Vault(ctx context.Context, customerID, nonce string) (processor.Card, error)
}

// lister reports the cards vaulted for a processor customer. Optional; used to
// heal stored rows that predate vaulting returning card facts.
type lister interface {
	Cards(ctx context.Context, customerID string) ([]processor.Card, error)
}

// finder finds the processor customer already made under a reference, "" when
// there is none. Optional; without it a subject with no saved card gets a
// customer made for it.
type finder interface {
	FindCustomer(ctx context.Context, reference string) (string, error)
}

// cardOnFile holds the durable identifiers for a vaulted card. A charge against
// the saved card needs BOTH: the token is the card id and the customer owns it.
type cardOnFile struct {
	CustomerID string         // the processor's customer id (one per subject)
	CardID     string         // the processor's reusable card token
	Card       processor.Card // card facts when the processor reports them (may be zero)
}

// cardVaultFrom returns the processor that vaults a NEW card for this org: the
// one the deployment's charge policy has take a card (the selection rule, Fiat),
// when it can hold cards on file. Stripe under the default policy.
func cardVaultFrom(reg *processor.Registry) (cardVault, bool) {
	p, err := reg.Fiat(context.Background(), processor.Need{Currency: currency.USD})
	if err != nil {
		return nil, false
	}
	cp, ok := p.(cardVault)
	if !ok || !cp.IsAvailable(context.Background()) {
		return nil, false
	}
	return cp, true
}

// cardVaultOf returns the processor a card ALREADY on file was vaulted with — a
// card saved before the deployment's policy changed stays with its processor,
// charged and detached there, regardless of the policy.
func cardVaultOf(ctx context.Context, reg *processor.Registry, pm *paymentmethod.PaymentMethod) (cardVault, bool) {
	p, err := reg.Existing(ctx, providerOf(pm))
	if err != nil {
		return nil, false
	}
	cp, ok := p.(cardVault)
	return cp, ok
}

// providerOf is the processor a payment method was saved with. Rows saved before
// processors were recorded are Square's.
func providerOf(pm *paymentmethod.PaymentMethod) processor.ProcessorType {
	if t := strings.ToLower(strings.TrimSpace(pm.ProviderType)); t != "" {
		return processor.ProcessorType(t)
	}
	return processor.Square
}

// existingCustomerID returns the customer id already associated with this
// subject's cards saved with processor via (one customer per subject), or "".
func existingCustomerID(db *datastore.Datastore, subject string, via processor.ProcessorType) string {
	rootKey := db.NewKey("synckey", "", 1, nil)
	iter := paymentmethod.Query(db).Ancestor(rootKey).Filter("CustomerId=", subject).Run()
	for {
		pm := paymentmethod.New(db)
		if _, err := iter.Next(pm); err != nil {
			break
		}
		if providerOf(pm) != via {
			continue
		}
		if id := customerIDOf(pm); id != "" {
			return id
		}
	}
	return ""
}

// customerIDOf is the processor customer that owns a vaulted card.
func customerIDOf(pm *paymentmethod.PaymentMethod) string {
	return firstNonEmpty(metaStr(pm, "customerId"), metaStr(pm, "squareCustomerId"))
}

// customerLocks serializes finding or making a subject's Square customer per
// (namespace, subject), so saves at once for one subject make one customer. It
// is its own lock, not defaultLock: it is held across processor calls, and the
// default lock guards only the default decision.
var customerLocks sync.Map // map[string]*sync.Mutex

func customerLock(db *datastore.Datastore, subject string) *sync.Mutex {
	m, _ := customerLocks.LoadOrStore(nscontext.GetNamespace(db.Context)+"\x00"+subject, &sync.Mutex{})
	return m.(*sync.Mutex)
}

// vaultCustomer returns the one customer of a billing subject, found or
// made once: the customer a saved card already names, else the customer Square
// holds under the subject's reference, else a new one made under it. The
// reference is the subject qualified by its namespace, as the saved cards and
// the locks are, because one Square account serves many orgs.
//
// Square indexes customers for search eventually, so a retry seconds after a
// create may not find it; the create's idempotency key, digested from the same
// request, answers that retry with the customer already made. Later retries find
// it by reference. A failed search is logged and the keyed create still runs, so
// a search outage never blocks a checkout.
func vaultCustomer(ctx context.Context, db *datastore.Datastore, cp cardVault, subject, email string) (string, error) {
	mu := customerLock(db, subject)
	mu.Lock()
	defer mu.Unlock()

	if id := existingCustomerID(db, subject, cp.Type()); id != "" {
		return id, nil
	}
	ref := nscontext.GetNamespace(db.Context) + "/" + subject
	if f, ok := cp.(finder); ok {
		switch id, err := f.FindCustomer(ctx, ref); {
		case err != nil:
			log.Warn("squareCustomer: search for %s failed, creating under its key: %v", ref, err)
		case id != "":
			return id, nil
		}
	}
	meta := map[string]interface{}{"reference": ref, "note": "Hanzo billing customer " + subject}
	id, err := cp.CreateCustomer(ctx, email, subject, meta)
	if err != nil {
		return "", fmt.Errorf("create square customer: %w", err)
	}
	return id, nil
}

// attachCardOnFile converts a single-use Square Web Payments nonce into a
// reusable card-on-file under customerID, the subject's one Square customer.
//
// Creating the card on file also validates the card with Square, so this both
// verifies AND saves in a single nonce use. A separate $1 pre-auth would consume
// the single-use nonce and make the subsequent attach fail — that's why the
// card-on-file path replaces verifyCardWithPreAuth rather than running after it.
func attachCardOnFile(ctx context.Context, cp cardVault, customerID, nonce string) (cardOnFile, error) {
	// Prefer the vault face that reports the card's facts; fall back to the
	// id-only attach for processors (and fakes) that don't have it.
	if v, ok := cp.(vaulter); ok {
		card, err := v.Vault(ctx, customerID, nonce)
		if err != nil {
			return cardOnFile{}, err
		}
		return cardOnFile{CustomerID: customerID, CardID: card.ID, Card: card}, nil
	}
	cardID, err := cp.AddPaymentMethod(ctx, customerID, nonce)
	if err != nil {
		return cardOnFile{}, err
	}
	return cardOnFile{CustomerID: customerID, CardID: cardID}, nil
}

// saveCard is the ONE constructor for a vaulted-card payment method: it vaults
// the nonce under the subject's one Square customer (squareCustomer), refuses
// to stack a duplicate, and persists a row a customer can recognize. Both
// endpoints that save a card — CreatePaymentMethod and SubscribeWithCard — go
// through it, so they cannot drift.
//
// Dedupe: the processor's Fingerprint identifies the card NUMBER across vaults.
// When the freshly-vaulted card matches a row the subject already holds (same
// fingerprint, or same brand+last4+expiry when a legacy row has no fingerprint),
// the new Square card-on-file is detached again (best-effort) and the EXISTING
// row is returned — saving the same card twice yields one row, structurally.
// created reports whether a NEW row was persisted; a decline after saveCard
// must only remove the Square card when created is true, or a retried checkout
// would destroy the customer's existing saved card.
func saveCard(ctx context.Context, db *datastore.Datastore, cp cardVault, subject, email, nonce string) (pm *paymentmethod.PaymentMethod, created bool, err error) {
	customerID, err := vaultCustomer(ctx, db, cp, subject, email)
	if err != nil {
		return nil, false, err
	}
	cof, err := attachCardOnFile(ctx, cp, customerID, nonce)
	if err != nil {
		return nil, false, err
	}

	if match := duplicateOf(db, subject, cof.Card); match != nil {
		// The vault call was still necessary — it is what validated the card and
		// revealed the fingerprint — but its product is redundant; detach it so
		// the Square vault holds one copy too. Best-effort: a failure leaves an
		// unreferenced card-on-file, never a wrong row.
		if rmErr := cp.RemovePaymentMethod(ctx, cof.CustomerID, cof.CardID); rmErr != nil {
			log.Warn("Failed to detach duplicate Square card %s: %v", cof.CardID, rmErr)
		}
		return match, false, nil
	}

	pm = paymentmethod.New(db)
	pm.CustomerId = subject
	pm.UserId = subject
	pm.Type = "card"
	pm.ProviderRef = cof.CardID
	pm.ProviderType = string(cp.Type())
	pm.Metadata = map[string]interface{}{
		"customerId": cof.CustomerID,
		"cardId":     cof.CardID,
		"verified":   true,
	}
	stampCard(pm, cof.Card)

	// The first card a subject saves is its default; a card saved while it has
	// one leaves that default alone. Decided and persisted under the subject's
	// default lock, so two saves at once cannot both find none. A default that
	// cannot be read is left alone too: in doubt, nothing is taken over.
	mu := defaultLock(db, subject)
	mu.Lock()
	defer mu.Unlock()
	held, herr := defaultsOf(db, subject)
	if herr != nil {
		log.Warn("saveCard: the default of %s could not be read, so the new card is not made default: %v", subject, herr)
	}
	pm.IsDefault = herr == nil && len(held) == 0
	if err := pm.Create(); err != nil {
		return nil, false, fmt.Errorf("persist payment method: %w", err)
	}
	return pm, true, nil
}

// duplicateOf returns the subject's existing payment-method row holding the
// same underlying card, or nil. Fingerprint is the key; brand+last4+expiry is
// the fallback for rows vaulted before fingerprints were stored. A zero card
// (no facts reported) can never match — absent evidence is not a duplicate.
func duplicateOf(db *datastore.Datastore, subject string, card processor.Card) *paymentmethod.PaymentMethod {
	if card.Fingerprint == "" && (card.Last4 == "" || card.Brand == "") {
		return nil
	}
	rootKey := db.NewKey("synckey", "", 1, nil)
	iter := paymentmethod.Query(db).Ancestor(rootKey).Filter("CustomerId=", subject).Run()
	for {
		pm := paymentmethod.New(db)
		if _, err := iter.Next(pm); err != nil {
			break
		}
		if !strings.EqualFold(strings.TrimSpace(pm.Type), "card") {
			continue
		}
		if fp := metaStr(pm, "fingerprint"); fp != "" && card.Fingerprint != "" {
			if fp == card.Fingerprint {
				return pm
			}
			continue
		}
		if pm.Card != nil && card.Brand != "" && card.Last4 != "" &&
			strings.EqualFold(pm.Card.Brand, card.Brand) && pm.Card.Last4 == card.Last4 &&
			pm.Card.ExpMonth == card.ExpMonth && pm.Card.ExpYear == card.ExpYear {
			return pm
		}
	}
	return nil
}

// stampCard writes the processor-reported card facts onto a row: the display
// details, the receipt name, and the fingerprint. A zero card stamps nothing.
func stampCard(pm *paymentmethod.PaymentMethod, card processor.Card) {
	if card.Brand != "" || card.Last4 != "" {
		pm.Card = &paymentmethod.CardDetails{
			Brand:    card.Brand,
			Last4:    card.Last4,
			ExpMonth: card.ExpMonth,
			ExpYear:  card.ExpYear,
		}
		if card.Brand != "" && card.Last4 != "" {
			pm.Name = card.Brand + " ending in " + card.Last4
		}
	}
	if card.Fingerprint != "" {
		if pm.Metadata == nil {
			pm.Metadata = map[string]interface{}{}
		}
		pm.Metadata["fingerprint"] = card.Fingerprint
	}
}

// heal backfills card facts onto rows vaulted before the processor reported
// them — the rows that render as four indistinguishable "card" entries. One
// processor list call per Square customer covers every such row; each healed
// row is persisted so the fetch happens once per row, ever. Best-effort by
// design: a processor error leaves the rows as they were.
func heal(ctx context.Context, reg *processor.Registry, methods []*paymentmethod.PaymentMethod) {
	var needy []*paymentmethod.PaymentMethod
	for _, pm := range methods {
		if strings.EqualFold(strings.TrimSpace(pm.Type), "card") && pm.Card == nil &&
			pm.ProviderRef != "" && customerIDOf(pm) != "" {
			needy = append(needy, pm)
		}
	}
	if len(needy) == 0 {
		return
	}
	// One list per processor customer (one per subject in practice).
	type owner struct {
		via      processor.ProcessorType
		customer string
	}
	byCustomer := map[owner][]*paymentmethod.PaymentMethod{}
	for _, pm := range needy {
		o := owner{providerOf(pm), customerIDOf(pm)}
		byCustomer[o] = append(byCustomer[o], pm)
	}
	for o, rows := range byCustomer {
		customerID := o.customer
		p, err := reg.Existing(ctx, o.via)
		if err != nil {
			continue
		}
		ls, ok := p.(lister)
		if !ok {
			continue
		}
		cards, err := ls.Cards(ctx, customerID)
		if err != nil {
			log.Warn("heal: list cards for %s failed: %v", customerID, err)
			continue
		}
		byID := map[string]processor.Card{}
		for _, c := range cards {
			byID[c.ID] = c
		}
		for _, pm := range rows {
			card, ok := byID[pm.ProviderRef]
			if !ok {
				continue
			}
			stampCard(pm, card)
			if err := pm.Update(); err != nil {
				log.Warn("heal: persist card facts on %s failed: %v", pm.Id(), err)
			}
		}
	}
}

// metaStr reads a string metadata value off a payment method, "" when absent.
func metaStr(pm *paymentmethod.PaymentMethod, key string) string {
	if pm.Metadata == nil {
		return ""
	}
	v, _ := pm.Metadata[key].(string)
	return strings.TrimSpace(v)
}

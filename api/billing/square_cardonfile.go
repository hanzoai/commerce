package billing

import (
	"context"
	"fmt"
	"strings"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/paymentmethod"
	"github.com/hanzoai/commerce/payment/processor"
)

// squareCustomerProcessor is the subset of processor.CustomerProcessor needed to
// vault a card on file: create/reuse a customer and attach/detach a card. The
// real *square.SquareProcessor satisfies this; the narrow interface keeps the
// billing handlers decoupled from the Square SDK and easy to unit test.
type squareCustomerProcessor interface {
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

// squareCardOnFile holds the durable identifiers for a vaulted Square card.
// A charge against the saved card needs BOTH: SourceID = CardID and the Square
// CustomerID (a card-on-file id is only chargeable in its customer's context).
type squareCardOnFile struct {
	CustomerID string         // Square customer id (one per org)
	CardID     string         // Square card-on-file id (reusable)
	Card       processor.Card // card facts when the processor reports them (may be zero)
}

// squareCustomerProcessorFrom returns the Square processor from a per-org
// registry as a squareCustomerProcessor, or (nil, false) when Square is not
// configured for the org or doesn't support customer/card-on-file management.
func squareCustomerProcessorFrom(reg *processor.Registry) (squareCustomerProcessor, bool) {
	p, err := reg.Get(processor.Square)
	if err != nil {
		return nil, false
	}
	cp, ok := p.(squareCustomerProcessor)
	if !ok || !cp.IsAvailable(context.Background()) {
		return nil, false
	}
	return cp, true
}

// existingSquareCustomerID returns the Square customer id already associated
// with this org's saved cards (one Square customer per org), or "" if none.
func existingSquareCustomerID(db *datastore.Datastore, customerID string) string {
	rootKey := db.NewKey("synckey", "", 1, nil)
	iter := paymentmethod.Query(db).Ancestor(rootKey).Filter("CustomerId=", customerID).Run()
	for {
		pm := paymentmethod.New(db)
		if _, err := iter.Next(pm); err != nil {
			break
		}
		if pm.Metadata == nil {
			continue
		}
		if v, ok := pm.Metadata["squareCustomerId"].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

// attachSquareCardOnFile converts a single-use Square Web Payments nonce into a
// reusable card-on-file. It reuses existingCustomerID when non-empty (one Square
// customer per org), otherwise creates a new Square customer identified by
// name/email.
//
// Creating the card on file also validates the card with Square, so this both
// verifies AND saves in a single nonce use. A separate $1 pre-auth would consume
// the single-use nonce and make the subsequent attach fail — that's why the
// card-on-file path replaces verifyCardWithPreAuth rather than running after it.
func attachSquareCardOnFile(ctx context.Context, cp squareCustomerProcessor, existingCustomerID, email, name, nonce string) (squareCardOnFile, error) {
	customerID := existingCustomerID
	if customerID == "" {
		meta := map[string]interface{}{}
		if name != "" {
			meta["note"] = "Hanzo billing customer " + name
		}
		id, err := cp.CreateCustomer(ctx, email, name, meta)
		if err != nil {
			return squareCardOnFile{}, fmt.Errorf("create square customer: %w", err)
		}
		customerID = id
	}

	// Prefer the vault face that reports the card's facts; fall back to the
	// id-only attach for processors (and fakes) that don't have it.
	if v, ok := cp.(vaulter); ok {
		card, err := v.Vault(ctx, customerID, nonce)
		if err != nil {
			return squareCardOnFile{}, err
		}
		return squareCardOnFile{CustomerID: customerID, CardID: card.ID, Card: card}, nil
	}
	cardID, err := cp.AddPaymentMethod(ctx, customerID, nonce)
	if err != nil {
		return squareCardOnFile{}, err
	}
	return squareCardOnFile{CustomerID: customerID, CardID: cardID}, nil
}

// saveCard is the ONE constructor for a vaulted-card payment method: it vaults
// the nonce, refuses to stack a duplicate, and persists a row a customer can
// recognize. Both doors that save a card — CreatePaymentMethod and
// SubscribeWithCard — go through it, so they cannot drift.
//
// Dedupe: the processor's Fingerprint identifies the card NUMBER across vaults.
// When the freshly-vaulted card matches a row the subject already holds (same
// fingerprint, or same brand+last4+expiry when a legacy row has no fingerprint),
// the new Square card-on-file is detached again (best-effort) and the EXISTING
// row is returned — saving the same card twice yields one row, structurally.
// created reports whether a NEW row was persisted; a decline after saveCard
// must only remove the Square card when created is true, or a retried checkout
// would destroy the customer's existing saved card.
func saveCard(ctx context.Context, db *datastore.Datastore, cp squareCustomerProcessor, subject, email, nonce string) (pm *paymentmethod.PaymentMethod, created bool, err error) {
	existing := existingSquareCustomerID(db, subject)
	cof, err := attachSquareCardOnFile(ctx, cp, existing, email, subject, nonce)
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
	pm.ProviderType = string(processor.Square)
	pm.Metadata = map[string]interface{}{
		"squareCustomerId": cof.CustomerID,
		"squareCardId":     cof.CardID,
		"squareVerified":   true,
	}
	stampCard(pm, cof.Card)
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
			pm.ProviderRef != "" && metaStr(pm, "squareCustomerId") != "" {
			needy = append(needy, pm)
		}
	}
	if len(needy) == 0 {
		return
	}
	p, err := reg.Get(processor.Square)
	if err != nil {
		return
	}
	ls, ok := p.(lister)
	if !ok {
		return
	}
	// One list per Square customer (one per org in practice).
	byCustomer := map[string][]*paymentmethod.PaymentMethod{}
	for _, pm := range needy {
		id := metaStr(pm, "squareCustomerId")
		byCustomer[id] = append(byCustomer[id], pm)
	}
	for customerID, rows := range byCustomer {
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

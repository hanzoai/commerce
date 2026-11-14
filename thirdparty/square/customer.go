package square

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	square "github.com/square/square-go-sdk/v3"
	"github.com/square/square-go-sdk/v3/customers"

	"github.com/hanzoai/commerce/payment/processor"
)

// CreateCustomer creates a customer profile in Square.
// Returns the Square customer ID on success.
//
// metadata["reference"] names what the customer is for: it is stored as the
// customer's reference_id, the field FindCustomer searches. The request carries
// an idempotency key digested from its own body, so a create sent again with
// the same inputs answers with the customer the first one made, and one key
// never travels with two different bodies.
func (sp *SquareProcessor) CreateCustomer(ctx context.Context, email, name string, metadata map[string]interface{}) (string, error) {
	req := &square.CreateCustomerRequest{
		EmailAddress: square.String(email),
	}

	// Split name into given/family; Square stores them separately.
	if name != "" {
		parts := strings.SplitN(name, " ", 2)
		req.GivenName = square.String(parts[0])
		if len(parts) == 2 {
			req.FamilyName = square.String(parts[1])
		}
	}

	// Attach metadata as a note (Square has no generic metadata field).
	if note, ok := metadata["note"].(string); ok && note != "" {
		req.Note = square.String(note)
	}
	if ref, ok := metadata["reference"].(string); ok && ref != "" {
		req.ReferenceID = square.String(fit(ref, refMax))
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("square create customer: %w", err)
	}
	sum := sha256.Sum256(body)
	req.IdempotencyKey = square.String(hex.EncodeToString(sum[:])[:keyMax])

	resp, err := sp.customersClient.Create(ctx, req)
	if err != nil {
		return "", fmt.Errorf("square create customer: %w", err)
	}
	if resp.Customer == nil || resp.Customer.ID == nil {
		return "", fmt.Errorf("square create customer: empty response")
	}
	return *resp.Customer.ID, nil
}

// FindCustomer returns the id of the oldest Square customer whose reference_id
// is reference, or "" when there is none. Square indexes customers for search
// eventually, so one created seconds ago may not be found yet.
func (sp *SquareProcessor) FindCustomer(ctx context.Context, reference string) (string, error) {
	resp, err := sp.customersClient.Search(ctx, &square.SearchCustomersRequest{
		Limit: square.Int64(1),
		Query: &square.CustomerQuery{
			Filter: &square.CustomerFilter{
				ReferenceID: &square.CustomerTextFilter{Exact: square.String(fit(reference, refMax))},
			},
			Sort: &square.CustomerSort{Field: square.CustomerSortFieldCreatedAt.Ptr()},
		},
	})
	if err != nil {
		return "", fmt.Errorf("square search customers: %w", err)
	}
	for _, c := range resp.Customers {
		if c != nil && c.ID != nil {
			return *c.ID, nil
		}
	}
	return "", nil
}

// GetCustomer retrieves a Square customer profile by ID.
func (sp *SquareProcessor) GetCustomer(ctx context.Context, customerID string) (map[string]interface{}, error) {
	resp, err := sp.customersClient.Get(ctx, &square.GetCustomersRequest{
		CustomerID: customerID,
	})
	if err != nil {
		return nil, fmt.Errorf("square get customer: %w", err)
	}
	if resp.Customer == nil {
		return nil, fmt.Errorf("square get customer: not found")
	}

	c := resp.Customer
	result := map[string]interface{}{
		"id": safeStr(c.ID),
	}
	if c.EmailAddress != nil {
		result["email"] = *c.EmailAddress
	}
	if c.GivenName != nil || c.FamilyName != nil {
		result["name"] = strings.TrimSpace(safeStr(c.GivenName) + " " + safeStr(c.FamilyName))
	}
	if c.Note != nil {
		result["note"] = *c.Note
	}
	if c.CreatedAt != nil {
		result["created_at"] = *c.CreatedAt
	}
	if c.UpdatedAt != nil {
		result["updated_at"] = *c.UpdatedAt
	}
	return result, nil
}

// UpdateCustomer updates mutable fields on a Square customer profile.
// Recognised keys: email, name, note.
func (sp *SquareProcessor) UpdateCustomer(ctx context.Context, customerID string, updates map[string]interface{}) error {
	req := &square.UpdateCustomerRequest{
		CustomerID: customerID,
	}

	if v, ok := updates["email"].(string); ok {
		req.EmailAddress = square.String(v)
	}
	if v, ok := updates["name"].(string); ok && v != "" {
		parts := strings.SplitN(v, " ", 2)
		req.GivenName = square.String(parts[0])
		if len(parts) == 2 {
			req.FamilyName = square.String(parts[1])
		}
	}
	if v, ok := updates["note"].(string); ok {
		req.Note = square.String(v)
	}

	_, err := sp.customersClient.Update(ctx, req)
	if err != nil {
		return fmt.Errorf("square update customer: %w", err)
	}
	return nil
}

// DeleteCustomer removes a Square customer profile.
func (sp *SquareProcessor) DeleteCustomer(ctx context.Context, customerID string) error {
	_, err := sp.customersClient.Delete(ctx, &square.DeleteCustomersRequest{
		CustomerID: customerID,
	})
	if err != nil {
		return fmt.Errorf("square delete customer: %w", err)
	}
	return nil
}

// AddPaymentMethod attaches a card nonce (token) to an existing Square customer.
// Returns the card-on-file ID. It is Vault reduced to the id — one vault call
// underneath, so the two can never disagree about what was saved.
func (sp *SquareProcessor) AddPaymentMethod(ctx context.Context, customerID, token string) (string, error) {
	card, err := sp.Vault(ctx, customerID, token)
	if err != nil {
		return "", err
	}
	return card.ID, nil
}

// Vault attaches a card nonce to a Square customer and returns the card as
// Square reports it — the durable id plus brand/last4/expiry (what a customer
// recognizes a card by) and the fingerprint (what identifies the card NUMBER
// across vaults, the dedupe key). Vaulting also validates the card, so a
// declined card errors here and nothing is saved.
func (sp *SquareProcessor) Vault(ctx context.Context, customerID, token string) (processor.Card, error) {
	resp, err := sp.customersClient.Cards.Create(ctx, &customers.CreateCustomerCardRequest{
		CustomerID: customerID,
		CardNonce:  token,
	})
	if err != nil {
		return processor.Card{}, fmt.Errorf("square add payment method: %w", err)
	}
	if resp.Card == nil || resp.Card.ID == nil {
		return processor.Card{}, fmt.Errorf("square add payment method: empty response")
	}
	card := cardOf(resp.Card)
	if card.CustomerID == "" {
		card.CustomerID = customerID
	}
	return card, nil
}

// Cards lists the cards vaulted for a Square customer (first page — Square caps
// at 25 per page and a billing customer holds a handful). Used to heal stored
// payment-method rows that predate Vault returning card details.
func (sp *SquareProcessor) Cards(ctx context.Context, customerID string) ([]processor.Card, error) {
	page, err := sp.cardsClient.List(ctx, &square.ListCardsRequest{
		CustomerID: square.String(customerID),
	})
	if err != nil {
		return nil, fmt.Errorf("square list cards: %w", err)
	}
	out := make([]processor.Card, 0, len(page.Results))
	for _, c := range page.Results {
		if c == nil || c.ID == nil {
			continue
		}
		out = append(out, cardOf(c))
	}
	return out, nil
}

// cardOf maps Square's wire Card onto the processor's value. One mapping, used
// by Vault and Cards, so a vaulted card and a listed card read identically.
func cardOf(c *square.Card) processor.Card {
	card := processor.Card{ID: safeStr(c.ID), Last4: safeStr(c.Last4), Fingerprint: safeStr(c.Fingerprint), CustomerID: safeStr(c.CustomerID)}
	if c.CardBrand != nil {
		card.Brand = brandName(string(*c.CardBrand))
	}
	if c.ExpMonth != nil {
		card.ExpMonth = int(*c.ExpMonth)
	}
	if c.ExpYear != nil {
		card.ExpYear = int(*c.ExpYear)
	}
	return card
}

// brandName renders Square's SCREAMING_SNAKE brand enum ("VISA",
// "AMERICAN_EXPRESS") as the display name a receipt prints ("Visa",
// "American Express").
func brandName(b string) string {
	words := strings.Split(strings.ToLower(b), "_")
	for i, w := range words {
		if w == "" {
			continue
		}
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}

// RemovePaymentMethod disables (deletes) a card on file from a Square customer.
func (sp *SquareProcessor) RemovePaymentMethod(ctx context.Context, customerID, paymentMethodID string) error {
	_, err := sp.customersClient.Cards.Delete(ctx, &customers.DeleteCardsRequest{
		CustomerID: customerID,
		CardID:     paymentMethodID,
	})
	if err != nil {
		return fmt.Errorf("square remove payment method: %w", err)
	}
	return nil
}

// safeStr dereferences a *string safely, returning "" for nil.
func safeStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// payerCustomer is the Square customer fields of p that are known.
func payerName(name string) (given, family *string) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, nil
	}
	parts := strings.SplitN(name, " ", 2)
	given = square.String(parts[0])
	if len(parts) == 2 && strings.TrimSpace(parts[1]) != "" {
		family = square.String(strings.TrimSpace(parts[1]))
	}
	return given, family
}

func payerAddress(a *processor.PayerAddress) *square.Address {
	if a.Empty() {
		return nil
	}
	out := &square.Address{}
	set := func(dst **string, v string) {
		if v = strings.TrimSpace(v); v != "" {
			*dst = square.String(v)
		}
	}
	set(&out.AddressLine1, a.Line1)
	set(&out.AddressLine2, a.Line2)
	set(&out.Locality, a.City)
	set(&out.AdministrativeDistrictLevel1, a.State)
	set(&out.PostalCode, a.Postal)
	if c := strings.ToUpper(strings.TrimSpace(a.Country)); c != "" {
		country := square.Country(c)
		out.Country = &country
	}
	return out
}

// CreateCustomerFor makes the payer's Square customer with everything known: given
// and family name, email, phone and billing address, the org in the note, and the
// reference FindCustomer searches. The key is digested from the body, as in
// CreateCustomer.
func (sp *SquareProcessor) CreateCustomerFor(ctx context.Context, p processor.Payer, reference string, metadata map[string]interface{}) (string, error) {
	req := &square.CreateCustomerRequest{}
	if e := strings.TrimSpace(p.Email); e != "" {
		req.EmailAddress = square.String(e)
	}
	req.GivenName, req.FamilyName = payerName(p.Name)
	if ph := strings.TrimSpace(p.Phone); ph != "" {
		req.PhoneNumber = square.String(ph)
	}
	req.Address = payerAddress(p.Address)
	note, _ := metadata["note"].(string)
	if p.Org != "" {
		note = strings.TrimSpace(note + " org=" + p.Org)
	}
	if note != "" {
		req.Note = square.String(note)
	}
	if reference != "" {
		req.ReferenceID = square.String(fit(reference, refMax))
	}
	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("square create customer: %w", err)
	}
	sum := sha256.Sum256(body)
	req.IdempotencyKey = square.String(hex.EncodeToString(sum[:])[:keyMax])
	resp, err := sp.customersClient.Create(ctx, req)
	if err != nil {
		return "", fmt.Errorf("square create customer: %w", err)
	}
	if resp.Customer == nil || resp.Customer.ID == nil {
		return "", fmt.Errorf("square create customer: empty response")
	}
	return *resp.Customer.ID, nil
}

// ProfileCustomer sets the fields of p the Square customer lacks or holds
// differently, in one update, and writes nothing when they agree.
func (sp *SquareProcessor) ProfileCustomer(ctx context.Context, customerID string, p processor.Payer) error {
	resp, err := sp.customersClient.Get(ctx, &square.GetCustomersRequest{CustomerID: customerID})
	if err != nil {
		return fmt.Errorf("square get customer: %w", err)
	}
	if resp.Customer == nil {
		return fmt.Errorf("square get customer: not found")
	}
	c := resp.Customer
	req := &square.UpdateCustomerRequest{CustomerID: customerID}
	changed := false
	if e := strings.TrimSpace(p.Email); e != "" && e != safeStr(c.EmailAddress) {
		req.EmailAddress, changed = square.String(e), true
	}
	if g, f := payerName(p.Name); g != nil && (*g != safeStr(c.GivenName) || safeStr(f) != safeStr(c.FamilyName)) {
		req.GivenName, req.FamilyName, changed = g, f, true
	}
	if ph := strings.TrimSpace(p.Phone); ph != "" && ph != safeStr(c.PhoneNumber) {
		req.PhoneNumber, changed = square.String(ph), true
	}
	if a := payerAddress(p.Address); a != nil {
		h := c.Address
		if h == nil || safeStr(h.AddressLine1) != safeStr(a.AddressLine1) || safeStr(h.Locality) != safeStr(a.Locality) ||
			safeStr(h.PostalCode) != safeStr(a.PostalCode) || (a.Country != nil && (h.Country == nil || *h.Country != *a.Country)) {
			req.Address, changed = a, true
		}
	}
	if !changed {
		return nil
	}
	if _, err := sp.customersClient.Update(ctx, req); err != nil {
		return fmt.Errorf("square update customer: %w", err)
	}
	return nil
}

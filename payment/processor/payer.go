package processor

import (
	"context"
	"strings"
)

// Payer is who is paying, as a processor needs to know them: the person, never the
// billing key they are charged under. Name and Email come from the host's verified
// identity; Phone and Address are what the buyer typed in the payment form. Org and
// User say whose books the payment is in, for the processor's own records.
type Payer struct {
	Name   string
	Email  string
	Phone  string
	Locale string // BCP 47, e.g. "en-US"; "" when unknown
	// Address is the billing address, nil when the buyer gave none.
	Address *PayerAddress
	Org     string
	User    string
}

// PayerAddress is a billing address in a processor's terms.
type PayerAddress struct {
	Line1, Line2, City, State, Postal, Country string
}

// Empty reports whether the address names nothing.
func (a *PayerAddress) Empty() bool {
	return a == nil || (a.Line1 == "" && a.Line2 == "" && a.City == "" && a.State == "" && a.Postal == "" && a.Country == "")
}

// Known reports whether the payer carries anything a processor can record.
func (p Payer) Known() bool {
	return strings.TrimSpace(p.Name) != "" || strings.TrimSpace(p.Email) != "" || strings.TrimSpace(p.Phone) != "" || !p.Address.Empty()
}

// PayerProfiles is a processor that keeps a customer profile of its payers: made
// once with everything known, and brought up to date at a later checkout when the
// profile is missing a field or holds a different one. The issuer and the
// processor's risk checks read the profile, so a payer the processor knows nothing
// about is declined more often.
type PayerProfiles interface {
	// CreateCustomerFor makes the payer's customer under reference (found again by
	// it) and returns its id.
	CreateCustomerFor(ctx context.Context, p Payer, reference string, metadata map[string]interface{}) (string, error)
	// ProfileCustomer sets the fields of p that the customer lacks or holds
	// differently. It writes nothing when the profile already matches.
	ProfileCustomer(ctx context.Context, customerID string, p Payer) error
}

// StatementSuffix is what follows the account's statement descriptor on a
// cardholder's statement, so the charge is recognized as Hanzo's.
const StatementSuffix = "HANZO AI"

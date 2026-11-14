package billing

import (
	"strings"

	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/paymentmethod"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/types"
)

// PayerIn is who is paying, as the host states them. Name and Email are the
// person's, from the verified identity the host resolved — never a value the request
// body chose. Phone and Address are what the buyer typed in the payment form. The
// billing key (the org slug, a user id) is never a name: a processor shows the name
// to the issuer, and an organization's slug declines.
type PayerIn struct {
	Name    string
	Email   string
	Phone   string
	Address *types.Address
}

// payerFor is p as the processors take it, for subject paying in org.
func payerFor(org *organization.Organization, subject string, p PayerIn) processor.Payer {
	out := processor.Payer{
		Name:  strings.TrimSpace(p.Name),
		Email: strings.TrimSpace(p.Email),
		Phone: strings.TrimSpace(p.Phone),
		User:  strings.ToLower(strings.TrimSpace(subject)),
	}
	if org != nil {
		out.Org = org.Name
	}
	if a := p.Address; a != nil && !a.Empty() {
		out.Address = &processor.PayerAddress{
			Line1: a.Line1, Line2: a.Line2, City: a.City, State: a.State,
			Postal: a.PostalCode, Country: a.Country,
		}
	}
	return out
}

// Facts a saved card keeps about its payer, so a charge made later with nobody
// present (a renewal, a reload) still names who paid.
const (
	factEmail = "payerEmail"
	factName  = "payerName"
	factOrg   = "payerOrg"
	factUser  = "payerUser"
)

func stampPayer(pm *paymentmethod.PaymentMethod, p processor.Payer) {
	if pm.Metadata == nil {
		pm.Metadata = map[string]interface{}{}
	}
	for k, v := range map[string]string{factEmail: p.Email, factName: p.Name, factOrg: p.Org, factUser: p.User} {
		if v != "" {
			pm.Metadata[k] = v
		}
	}
}

// payerOfCard is the payer a saved card names.
func payerOfCard(pm *paymentmethod.PaymentMethod) processor.Payer {
	return processor.Payer{
		Email: metaStr(pm, factEmail), Name: metaStr(pm, factName),
		Org: metaStr(pm, factOrg), User: metaStr(pm, factUser),
	}
}

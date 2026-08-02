// Reference: https://developer.paypal.com/webapps/developer/docs/classic/ipn/integration-guide/IPNIntro/
package ipn

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/hanzoai/commerce/models/types/currency"
)

type IpnMessage struct {
	Status     string         // transaction[0].status
	PayerEmail string         // sender_email
	PayeeEmail string         // transaction[0].receiver
	Amount     currency.Cents // extracted from transaction[0].amount
	PayKey     string         // pay_key
	Currency   currency.Type
}

// NewIpnMessage parses a PayPal IPN form.
//
// It returns an error rather than a zero-amount message: an IPN whose amount
// does not parse is a payment notification we cannot attribute, and recording
// it as 0 would post a free transaction that reports success. The amount field
// is "&lt;CURRENCY&gt; &lt;decimal&gt;", so a field with no space is malformed too — it
// used to index parts[1] unguarded and panic.
func NewIpnMessage(form url.Values) (*IpnMessage, error) {
	message := new(IpnMessage)
	message.Status = form.Get("transaction[0].status")
	message.PayerEmail = form.Get("sender_email")
	message.PayeeEmail = form.Get("transaction[0].receiver")
	message.PayKey = form.Get("pay_key")

	raw := form.Get("transaction[0].amount")
	parts := strings.Fields(raw)
	if len(parts) != 2 {
		return nil, fmt.Errorf("ipn: amount %q is not \"&lt;CURRENCY&gt; &lt;decimal&gt;\"", raw)
	}

	amount, err := currency.CentsFromString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("ipn: %w", err)
	}
	message.Amount = amount
	message.Currency = currency.Type(strings.ToLower(parts[0]))

	return message, nil
}

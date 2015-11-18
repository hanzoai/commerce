// Reference: https://developer.paypal.com/webapps/developer/docs/classic/ipn/integration-guide/IPNIntro/
package ipn

import (
	"net/url"
	"strings"

	"crowdstart.com/models/types/currency"
)

type IpnMessage struct {
	Status     string         // transaction[0].status
	PayerEmail string         // sender_email
	PayeeEmail string         // transaction[0].receiver
	Amount     currency.Cents // extracted from transaction[0].amount
	PayKey     string         // pay_key
	Currency   string
}

func NewIpnMessage(form url.Values) *IpnMessage {
	message := new(IpnMessage)
	message.Status = form.Get("transaction[0].status")
	message.PayerEmail = form.Get("sender_email")
	message.PayeeEmail = form.Get("transaction[0].receiver")
	message.PayKey = form.Get("pay_key")

	amount := form.Get("transaction[0].amount")
	parts := strings.Split(amount, " ")

	message.Amount = currency.CentsFromString(parts[1])
	message.Currency = parts[0]

	return message
}

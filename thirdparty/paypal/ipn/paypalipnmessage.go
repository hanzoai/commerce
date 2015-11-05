// Reference: https://developer.paypal.com/webapps/developer/docs/classic/ipn/integration-guide/IPNIntro/
package ipn

import "crowdstart.com/models/types/currency"

type PayPalIpnMessage struct {
	Status     string         // status
	PayerEmail string         // sender_email
	PayeeEmail string         // transaction[0].receiver
	Amount     currency.Cents // extracted from transaction[0].amount
	PayKey     string         // pay_key
}

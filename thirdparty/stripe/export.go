package stripe

import (
	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/charge"
	"github.com/stripe/stripe-go/dispute"
)

type Card stripe.Card
type CardParams stripe.CardParams
type Charge stripe.Charge
type ChargeListParams stripe.ChargeListParams
type ChargeParams stripe.ChargeParams
type Customer stripe.Customer
type Dispute stripe.Dispute
type Event struct {
	ID       string            `json:"id"`
	Account  string            `json:"account"`
	Live     bool              `json:"livemode"`
	Created  int64             `json:"created"`
	Data     *stripe.EventData `json:"data"`
	Webhooks uint64            `json:"pending_webhooks"`
	Type     string            `json:"type"`
	Request  string            `json:"request"`
}
type Sub stripe.Sub

type Reversal stripe.Reversal
type Token stripe.Token
type Transfer stripe.Transfer
type Payout stripe.Payout

const ReportFraudulent = charge.ReportFraudulent
const ReportSafe = charge.ReportSafe

const Won = dispute.Won
const ChargeRefunded = dispute.ChargeRefunded
const Lost = dispute.Lost
const Review = dispute.Review

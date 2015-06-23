package stripe

import (
	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/charge"
	"github.com/stripe/stripe-go/dispute"
)

type Card stripe.Card
type CardParams stripe.CardParams
type Charge stripe.Charge
type ChargeParams stripe.ChargeParams
type ChargeListParams stripe.ChargeListParams
type Customer stripe.Customer
type Dispute stripe.Dispute
type Token stripe.Token
type Event stripe.Event

const ReportFraudulent = charge.ReportFraudulent
const ReportSafe = charge.ReportSafe

const Won = dispute.Won
const ChargeRefunded = dispute.ChargeRefunded
const Lost = dispute.Lost
const Review = dispute.Review

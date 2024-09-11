package stripe

import (
	_stripe "github.com/stripe/stripe-go/v75"
)

type Card _stripe.Card
type CardParams _stripe.CardParams
type Charge _stripe.Charge
type ChargeListParams _stripe.ChargeListParams
type ChargeParams _stripe.ChargeParams
type Customer _stripe.Customer
type Dispute _stripe.Dispute
type Plan _stripe.Plan
type PlanListParams _stripe.PlanListParams
type PlanParams _stripe.PlanParams
type PlanList _stripe.PlanList
type Event struct {
	ID       string             `json:"id"`
	Account  string             `json:"account"`
	Live     bool               `json:"livemode"`
	Created  int64              `json:"created"`
	Data     *_stripe.EventData `json:"data"`
	Webhooks uint64             `json:"pending_webhooks"`
	Type     string             `json:"type"`
	Request  string             `json:"request"`
}
type Subscription _stripe.Subscription

type Reversal _stripe.TransferReversal
type Token _stripe.Token
type Transfer _stripe.Transfer
type Payout _stripe.Payout

const ReportSafe = _stripe.ChargeFraudUserReportSafe
const ReportFraudulent = _stripe.ChargeFraudUserReportFraudulent

const Won = _stripe.DisputeStatusWon
const Lost = _stripe.DisputeStatusLost
const Review = _stripe.DisputeStatusUnderReview
const Needsresponse = _stripe.DisputeStatusNeedsResponse

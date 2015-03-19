package models

import (
	stripe "github.com/stripe/stripe-go"
)

// The datastore doesn't support unsigned integers
// int64 is used for amounts instead
type Dispute struct {
	Live         bool                  `json:"livemode"`
	Amount       int64                 `json:"amount"`
	Currency     stripe.Currency       `json:"currency"`
	Charge       string                `json:"charge"`
	Created      int64                 `json:"created"`
	Refundable   bool                  `json:"is_charge_refundable"`
	Reason       stripe.DisputeReason  `json:"reason"`
	Status       stripe.DisputeStatus  `json:"status"`
	Transactions []*stripe.Transaction `json:"balance_transactions"`
	// Evidence        *stripe.DisputeEvidence `json:"evidence"`
	// EvidenceDetails *stripe.EvidenceDetails `json:"evidence_details"`
	// Meta map[string]string `json:"metadata"`
}

// casts Dispute.Amount to int64
func ConvertDispute(d stripe.Dispute) (n Dispute) {
	n.Live = d.Live
	n.Amount = int64(d.Amount)
	n.Currency = d.Currency
	n.Charge = d.Charge
	n.Created = d.Created
	n.Refundable = d.Refundable
	n.Reason = d.Reason
	n.Status = d.Status
	n.Transactions = d.Transactions
	// n.Evidence = d.Evidence
	// n.EvidenceDetails = d.EvidenceDetails
	// n.Meta = d.Meta
	return n
}

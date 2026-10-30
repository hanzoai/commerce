// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package circle

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// The batched-payment half of Gateway (gateway-nanopayments.yaml): a payer
// signs an EIP-3009 authorization against their Gateway balance, Gateway
// verifies it, locks the amount, queues it for batched onchain settlement and
// credits the payee's Gateway balance when the batch lands.
//
// Settle is used rather than POST /v1/batch/submit, which carries the same
// authorization to the same queue. Settle takes the payee's REQUIREMENTS
// beside the payment, so Gateway itself refuses a payment whose `to` is not
// the payee (address_mismatch) or whose value is not the amount
// (amount_mismatch); it answers every refusal with a typed errorReason rather
// than a bare 400; and its transaction id is the one GET /v1/x402/transfers/{id}
// reads, which is how an accepted payment is followed to settlement.

// Scheme is the x402 scheme Gateway batches: the payer authorizes EXACTLY the
// amount. Version is the x402 protocol version of its payloads.
const (
	Scheme  = "exact"
	Version = 2
)

// Payment states, as GET /v1/x402/transfers reports them.
const (
	Received  = "received"  // accepted; the payer's balance is locked
	Batched   = "batched"   // included in a batch
	Confirmed = "confirmed" // the batch landed onchain
	Completed = "completed" // fully complete
	Failed    = "failed"    // never charged
)

// Kind is one network Gateway batches on, from GET /v1/x402/supported.
type Kind struct {
	Version int    `json:"x402Version"`
	Scheme  string `json:"scheme"`
	Network string `json:"network"` // CAIP-2, "eip155:<chain id>"
	Extra   Extra  `json:"extra"`
}

// Extra is what a payer needs to sign on that network: the EIP-712 domain
// (name, version, verifyingContract — the GatewayWallet contract), the
// shortest validity Gateway accepts, and the tokens it batches.
type Extra struct {
	Name              string  `json:"name"`
	Version           string  `json:"version"`
	VerifyingContract string  `json:"verifyingContract"`
	MinValidity       int64   `json:"minValiditySeconds"`
	Assets            []Asset `json:"assets"`
}

// Asset is a token Gateway batches on one network. Versions are the
// authorization-domain versions it accepts for the token: "1" signs the
// four-field domain, "2" adds the token address as salt.
type Asset struct {
	Symbol   string   `json:"symbol"`
	Address  string   `json:"address"`
	Decimals int      `json:"decimals"`
	Versions []string `json:"authorizationVersions"`
}

// Supported reads the networks and tokens Gateway batches, live.
func (c *Client) Supported(ctx context.Context) ([]Kind, error) {
	var out struct {
		Kinds []Kind `json:"kinds"`
	}
	err := c.do(ctx, http.MethodGet, "/v1/x402/supported", nil, nil, &out)
	return out.Kinds, err
}

// Signing is the `extra` of a requirement: the EIP-712 domain the payer signed
// under, minus the chain id, which the network carries.
type Signing struct {
	Name              string `json:"name"`
	Version           string `json:"version"`
	VerifyingContract string `json:"verifyingContract"`
}

// Requirements is what the payee asks for (x402 PaymentRequirements).
type Requirements struct {
	Scheme            string  `json:"scheme"`
	Network           string  `json:"network"`
	Asset             string  `json:"asset"`
	Amount            string  `json:"amount"` // base units
	PayTo             string  `json:"payTo"`
	MaxTimeoutSeconds int64   `json:"maxTimeoutSeconds"`
	Extra             Signing `json:"extra"`
}

// Payload is the payer's half: the authorization and its signature.
type Payload struct {
	Authorization Authorization `json:"authorization"`
	Signature     string        `json:"signature"`
}

// Resource names what a payment bought. Gateway requires it, all three fields,
// on every payment: its OpenAPI marks resource optional and only url required
// within it, and the live API answers 400 "paymentPayload.resource: Required",
// then "resource.description: Required, resource.mimeType: Required"
// (measured on testnet).
type Resource struct {
	URL         string `json:"url"`
	Description string `json:"description"`
	MimeType    string `json:"mimeType"`
}

// Settlement is Gateway's answer to a settle. Success means Gateway verified
// the signature and LOCKED the payer's balance; the payment is now queued and
// Transaction names it. On a refusal Reason is Gateway's errorReason
// (insufficient_balance, nonce_already_used, invalid_signature, …).
type Settlement struct {
	Success     bool   `json:"success"`
	Reason      string `json:"errorReason,omitempty"`
	Payer       string `json:"payer,omitempty"`
	Transaction string `json:"transaction"`
	Network     string `json:"network"`
}

// Settle submits a signed authorization for batched settlement against req.
//
// A 200 is an answer either way and comes back as a Settlement. Anything else
// comes back as an error, and only Refused(err) means Gateway processed
// nothing; a timeout or a 5xx is an outcome the caller does not know.
func (c *Client) Settle(ctx context.Context, r Resource, req Requirements, p Payload) (Settlement, error) {
	var out Settlement
	err := c.do(ctx, http.MethodPost, "/v1/x402/settle", nil, submission(r, req, p), &out)
	return out, err
}

// submission is the body settle and verify both take: the payment as the
// payer presents it (x402 PaymentPayload) and the payee's requirements.
func submission(r Resource, req Requirements, p Payload) any {
	type payment struct {
		Version  int          `json:"x402Version"`
		Resource Resource     `json:"resource"`
		Accepted Requirements `json:"accepted"`
		Payload  Payload      `json:"payload"`
	}
	return struct {
		Payment      payment      `json:"paymentPayload"`
		Requirements Requirements `json:"paymentRequirements"`
	}{payment{Version, r, req, p}, req}
}

// Payment is a batched payment as Gateway records it. Amount is in base
// units; TxHash is the batch's settlement transaction, empty until there is
// one.
type Payment struct {
	ID               string `json:"id"`
	Status           string `json:"status"`
	Token            string `json:"token"`
	SendingNetwork   string `json:"sendingNetwork"`
	RecipientNetwork string `json:"recipientNetwork"`
	From             string `json:"fromAddress"`
	To               string `json:"toAddress"`
	Amount           string `json:"amount"`
	Nonce            string `json:"nonce"`
	TxHash           string `json:"txHash"`
	CreatedAt        string `json:"createdAt"`
	UpdatedAt        string `json:"updatedAt"`
}

// Payment reads one batched payment by the id Settle answered with.
func (c *Client) Payment(ctx context.Context, id string) (Payment, error) {
	var out Payment
	if !uuid(id) {
		return out, fmt.Errorf("circle gateway: payment id %q is not a UUID", id)
	}
	err := c.do(ctx, http.MethodGet, "/v1/x402/transfers/"+url.PathEscape(id), nil, nil, &out)
	return out, err
}

// Find reads the payments one payer made with one nonce on one network.
// Gateway refuses a second use of a (payer, nonce), so a real answer holds at
// most one payment; it is a list because that is what the endpoint returns.
func (c *Client) Find(ctx context.Context, network, from, nonce string) ([]Payment, error) {
	var out struct {
		Transfers []Payment `json:"transfers"`
	}
	q := url.Values{}
	q.Set("network", network)
	q.Set("from", from)
	q.Set("nonce", nonce)
	q.Set("pageSize", "10")
	err := c.do(ctx, http.MethodGet, "/v1/x402/transfers", q, nil, &out)
	return out.Transfers, err
}

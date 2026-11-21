// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

// Package x402 is pay-per-request over HTTP 402: a resource names its price, a
// client signs for exactly that price, the resource is served, and the payment
// settles. It speaks x402 protocol version 2 as published in
// github.com/x402-foundation/x402: specs/x402-specification-v2.md, the HTTP
// transport in specs/transports-v2/http.md, and the exact scheme on EVM in
// specs/schemes/exact/scheme_exact_evm.md.
//
// The pieces compose and each stands alone:
//
//   - wire types and the base64 JSON header codec (this file);
//   - Verify and InWindow, the offline checks on a payment (check.go);
//   - Sign, the client half of the same EIP-712 encoding (sign.go);
//   - Facilitator, the verify and settle interface of spec section 7, and
//     Remote, a facilitator reached over HTTP (facilitator.go);
//   - Table, the resource price table over one or more assets (table.go);
//   - Gate, the resource server's flow: challenge, verify, serve, settle
//     (gate.go), and Middleware and Transport for net/http (http.go).
//
// Prices, payees, assets and networks are configuration: nothing here names a
// token, a chain or an amount. Only the exact scheme with the eip3009 asset
// transfer method is implemented; permit2 and ERC-7710 transfer methods, the
// upto and batch schemes, extensions and the discovery API are not.
package x402

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

const (
	// Version is the protocol version this package speaks. It is a number on the
	// wire (`x402Version: 2`) and is the only thing that decides how a message
	// is read.
	Version = 2

	// SchemeExact authorizes exactly the advertised amount (spec section 6).
	SchemeExact = "exact"

	// TransferEIP3009 is the exact-scheme EVM asset transfer method this package
	// verifies: the token's own transferWithAuthorization. It is the default when
	// `extra.assetTransferMethod` is absent (scheme_exact_evm.md).
	TransferEIP3009 = "eip3009"

	// HeaderPaymentRequired carries the base64 PaymentRequired on a 402.
	HeaderPaymentRequired = "PAYMENT-REQUIRED"
	// HeaderPaymentSignature carries the client's base64 PaymentPayload on the retry.
	HeaderPaymentSignature = "PAYMENT-SIGNATURE"
	// HeaderPaymentResponse carries the base64 SettlementResponse.
	HeaderPaymentResponse = "PAYMENT-RESPONSE"
)

// PaymentRequired is the 402 challenge: the resource, and every way it may be
// paid for (spec 5.1).
type PaymentRequired struct {
	X402Version int                   `json:"x402Version"`
	Error       string                `json:"error,omitempty"`
	Resource    ResourceInfo          `json:"resource"`
	Accepts     []PaymentRequirements `json:"accepts"`
	Extensions  Extensions            `json:"extensions,omitempty"`
}

// ResourceInfo describes the protected resource (spec 5.1.2).
type ResourceInfo struct {
	URL         string   `json:"url"`
	Description string   `json:"description,omitempty"`
	MimeType    string   `json:"mimeType,omitempty"`
	ServiceName string   `json:"serviceName,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	IconURL     string   `json:"iconUrl,omitempty"`
}

// PaymentRequirements is one acceptable way to pay: the exact amount of the
// exact asset on the exact network that must reach exactly this payee.
type PaymentRequirements struct {
	Scheme            string `json:"scheme"`
	Network           string `json:"network"` // CAIP-2, e.g. "eip155:8453"
	Amount            string `json:"amount"`  // atomic units of Asset, decimal
	Asset             string `json:"asset"`   // token contract address
	PayTo             string `json:"payTo"`
	MaxTimeoutSeconds int64  `json:"maxTimeoutSeconds"`
	Extra             *Extra `json:"extra,omitempty"`
}

// Extra is the exact-scheme EVM `extra`: the token's EIP-712 domain name and
// version, which is what makes a signature verifiable at all, and the transfer
// method it was signed for (scheme_exact_evm.md, eip3009).
type Extra struct {
	AssetTransferMethod string `json:"assetTransferMethod,omitempty"`
	Name                string `json:"name"`
	Version             string `json:"version"`
}

// PaymentPayload is the client's payment (spec 5.2). Accepted echoes which of
// the offered requirements the client chose; it is compared with what was
// offered and never trusted as the terms.
type PaymentPayload struct {
	X402Version int                 `json:"x402Version"`
	Resource    *ResourceInfo       `json:"resource,omitempty"`
	Accepted    PaymentRequirements `json:"accepted"`
	Payload     ExactPayload        `json:"payload"`
	Extensions  Extensions          `json:"extensions,omitempty"`
}

// ExactPayload is the exact-scheme EVM payload: the signature and the
// parameters needed to rebuild the message it signed.
type ExactPayload struct {
	Signature     string        `json:"signature"`
	Authorization Authorization `json:"authorization"`
}

// SettlementResponse is the outcome of settling a payment (spec 5.3). It rides
// PAYMENT-RESPONSE on the success leg and on the failure leg.
type SettlementResponse struct {
	Success      bool       `json:"success"`
	ErrorReason  string     `json:"errorReason,omitempty"`
	ErrorMessage string     `json:"errorMessage,omitempty"`
	Payer        string     `json:"payer,omitempty"`
	Transaction  string     `json:"transaction"`
	Network      string     `json:"network"`
	Amount       string     `json:"amount,omitempty"`
	Extensions   Extensions `json:"extensions,omitempty"`
}

// VerifyResponse is a facilitator's answer to POST /verify (spec 5.4).
type VerifyResponse struct {
	IsValid        bool            `json:"isValid"`
	InvalidReason  string          `json:"invalidReason,omitempty"`
	InvalidMessage string          `json:"invalidMessage,omitempty"`
	Payer          string          `json:"payer,omitempty"`
	Extensions     Extensions      `json:"extensions,omitempty"`
	Extra          json.RawMessage `json:"extra,omitempty"`
}

// SupportedResponse is a facilitator's answer to GET /supported (spec 7.3).
type SupportedResponse struct {
	Kinds      []SupportedKind     `json:"kinds"`
	Extensions []string            `json:"extensions"`
	Signers    map[string][]string `json:"signers"`
}

// SupportedKind is one scheme and network a facilitator settles.
type SupportedKind struct {
	X402Version int             `json:"x402Version"`
	Scheme      string          `json:"scheme"`
	Network     string          `json:"network"`
	Extra       json.RawMessage `json:"extra,omitempty"`
}

// Extensions is the protocol's extension map, carried through verbatim: an
// unknown extension is preserved, never dropped.
type Extensions map[string]json.RawMessage

// EncodeHeader renders v as the base64 JSON an x402 header carries: standard,
// padded base64, which is what the specification's examples decode as.
func EncodeHeader(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("x402: encode header: %w", err)
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

// DecodeHeader reads a base64 JSON header value into v. Unpadded and URL-safe
// base64 are accepted as well: each is unambiguous here, and a client library
// reaches for them by accident.
func DecodeHeader(header string, v any) error {
	s := strings.TrimSpace(header)
	if s == "" {
		return fmt.Errorf("x402: empty header")
	}
	var raw []byte
	var err error
	for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		if raw, err = enc.DecodeString(s); err == nil {
			break
		}
	}
	if err != nil {
		return fmt.Errorf("x402: header is not base64: %w", err)
	}
	if err := json.Unmarshal(raw, v); err != nil {
		return fmt.Errorf("x402: header is not %T: %w", v, err)
	}
	return nil
}

// ParsePayment decodes a PaymentPayload from a PAYMENT-SIGNATURE value and
// requires the fields every check below reads.
func ParsePayment(header string) (PaymentPayload, error) {
	var p PaymentPayload
	if err := DecodeHeader(header, &p); err != nil {
		return p, &Error{Reason: ReasonInvalidPayload, Message: err.Error()}
	}
	a := p.Payload.Authorization
	if a.From == "" || a.To == "" || a.Value == "" || a.Nonce == "" || p.Payload.Signature == "" {
		return p, &Error{Reason: ReasonInvalidPayload,
			Message: "payload.signature and payload.authorization.{from,to,value,nonce} are required"}
	}
	return p, nil
}

// ChainID is the EIP-155 chain id a CAIP-2 network identifier names. The
// network is one value on the wire and the chain id is read out of it: it is
// the chainId in the token's EIP-712 domain, so it is not a label.
func ChainID(network string) (int64, error) {
	ns, ref, ok := strings.Cut(strings.TrimSpace(network), ":")
	if !ok || !caip2(ns, 3, 8, false) || !caip2(ref, 1, 32, true) {
		return 0, &Error{Reason: ReasonInvalidNetwork,
			Message: fmt.Sprintf("network %q is not CAIP-2 (namespace:reference, e.g. eip155:8453)", network)}
	}
	if ns != "eip155" {
		return 0, &Error{Reason: ReasonInvalidNetwork,
			Message: fmt.Sprintf("network %q is in namespace %q; EIP-3009 settles on eip155 only", network, ns)}
	}
	id, err := strconv.ParseInt(ref, 10, 64)
	if err != nil || id <= 0 {
		return 0, &Error{Reason: ReasonInvalidNetwork, Message: fmt.Sprintf("network %q has no eip155 chain id", network)}
	}
	return id, nil
}

// caip2 checks a CAIP-2 token: the namespace is [-a-z0-9]{3,8}, the reference
// [-_a-zA-Z0-9]{1,32}.
func caip2(s string, min, max int, ref bool) bool {
	if len(s) < min || len(s) > max {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-':
		case ref && (r >= 'A' && r <= 'Z' || r == '_'):
		default:
			return false
		}
	}
	return true
}

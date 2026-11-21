// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

// Package ucp is the business side of the Universal Commerce Protocol (UCP): the
// discovery profile served at /.well-known/ucp, capability negotiation, the
// Checkout capability over the REST transport, RFC 9421 message signatures, and
// the Order capability's webhook and Get Order.
//
// The protocol is Universal-Commerce-Protocol/ucp, Apache-2.0. This package
// implements release Version, checked against commit c20aa35c2c91 of that
// repository: human specification in docs/specification, machine schemas in
// source/schemas and source/services. Section citations in comments name those
// documents (overview = docs/specification/overview/index.md).
//
// The package is plain net/http. A host supplies who the caller is (Identify),
// what is sold (Backend) and where it is served (Config); nothing is read from
// the environment.
package ucp

import (
	"fmt"
	"net/http"
)

// Version is the protocol release this package speaks and advertises as
// ucp.version. It is the latest dated release in the specification's own
// announcements (v2026-08-25); the overview forbids advertising a draft
// ("Pre-release Versions"), so the date of the release is what is served.
const Version = "2026-08-25"

// Source is the specification snapshot the implementation was checked against.
const Source = "Universal-Commerce-Protocol/ucp@c20aa35c2c91 (Apache-2.0)"

// Names the specification defines (overview, "Standard Capabilities";
// shopping/checkout, shopping/order).
const (
	Shopping   = "dev.ucp.shopping"
	Checkout   = "dev.ucp.shopping.checkout"
	OrderCap   = "dev.ucp.shopping.order"
	WellKnown  = "/.well-known/ucp"
	TransportR = "rest"
)

// Request headers (shopping/rest.openapi.json, components.parameters).
const (
	HeaderAgent       = "UCP-Agent"
	HeaderIdempotency = "Idempotency-Key"
	HeaderRequestID   = "Request-Id"
	HeaderWebhookID   = "Webhook-Id"
	HeaderWebhookTime = "Webhook-Timestamp"
)

// Error codes of the negotiation, signature and profile failures
// (overview, "Error Codes"; signatures, "Error Handling").
const (
	CodeInvalidProfileURL        = "invalid_profile_url"
	CodeProfileUnreachable       = "profile_unreachable"
	CodeProfileMalformed         = "profile_malformed"
	CodeProfileNotTrusted        = "profile_not_trusted"
	CodeVersionUnsupported       = "version_unsupported"
	CodeCapabilitiesIncompatible = "capabilities_incompatible"
	CodeSignatureMissing         = "signature_missing"
	CodeSignatureInvalid         = "signature_invalid"
	CodeKeyNotFound              = "key_not_found"
	CodeDigestMismatch           = "digest_mismatch"
	CodeAlgorithmUnsupported     = "algorithm_unsupported"
)

// statusOf is the REST status of each registered code (overview, "Error Codes").
var statusOf = map[string]int{
	CodeInvalidProfileURL:    http.StatusBadRequest,
	CodeProfileUnreachable:   http.StatusFailedDependency,
	CodeProfileMalformed:     http.StatusUnprocessableEntity,
	CodeProfileNotTrusted:    http.StatusForbidden,
	CodeVersionUnsupported:   http.StatusUnprocessableEntity,
	CodeSignatureMissing:     http.StatusUnauthorized,
	CodeSignatureInvalid:     http.StatusUnauthorized,
	CodeKeyNotFound:          http.StatusUnauthorized,
	CodeDigestMismatch:       http.StatusBadRequest,
	CodeAlgorithmUnsupported: http.StatusBadRequest,
}

// Error is a UCP transport error: the {code, content, continue_url} body REST
// answers with for discovery, version, signature and protocol failures
// (overview, "Transport Bindings", REST).
type Error struct {
	Code        string `json:"code"`
	Content     string `json:"content"`
	ContinueURL string `json:"continue_url,omitempty"`
	// Status is the HTTP status the error is answered with.
	Status int `json:"-"`
	// RetryAfter, in seconds, is sent as Retry-After on 429 and 503.
	RetryAfter int `json:"-"`
}

func (e *Error) Error() string { return e.Code + ": " + e.Content }

// fail builds an Error for a registered code, or for a protocol status.
func fail(code string, format string, args ...any) *Error {
	st := statusOf[code]
	if st == 0 {
		st = http.StatusBadRequest
	}
	return &Error{Code: code, Status: st, Content: fmt.Sprintf(format, args...)}
}

// status builds an Error for a protocol status (400, 401, 403, 409, 429, 500, 503).
func status(st int, code, format string, args ...any) *Error {
	return &Error{Code: code, Status: st, Content: fmt.Sprintf(format, args...)}
}

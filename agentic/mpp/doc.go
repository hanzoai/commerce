// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

// Package mpp is the Machine Payments Protocol: the "Payment" HTTP authentication
// scheme, an HTTP 402 challenge answered by an Authorization credential and
// acknowledged by a Payment-Receipt.
//
// Status of the specification. The scheme is an IETF Internet-Draft, not a standard:
// draft-httpauth-payment-01 (9 September 2026, Tempo Labs and Stripe), a working
// document that expires 13 March 2027. It is implemented from the published draft and
// from the editors' working copy in github.com/tempoxyz/mpp-specs (commit 7bb99c85,
// 11 November 2026), which is later than the published text. The intent and method
// drafts used are draft-payment-intent-charge-00, draft-stripe-charge-00 and
// draft-payment-discovery-01. Every one of them can change; none is a stable contract.
//
// Where the working copy differs from the published draft, this package follows the
// working copy, and says so at the site:
//
//   - The HMAC-SHA256 challenge binding (draft section 5.1.2.1.1). The published text
//     puts the header slot last, after opaque; the working copy inserts it before
//     opaque and publishes test vectors, which testdata/core.md carries and the tests
//     reproduce. A challenge without a header parameter binds identically in both.
//   - The problem types bad-request, invalid-payload, internal-payment-error and
//     payment-action-required, which the published table lacks.
//   - Other authentication is verified before a Payment credential is consumed.
//
// What the Gate adds where the draft leaves room. The binding does not cover the request
// being paid for or who pays, so each challenge carries in opaque the route (method and
// path) and a keyed tag of the caller, and a credential is accepted only for the same
// route and caller, and for the amount, currency and external reference the host now
// names. The binding is deterministic, so opaque also carries a random nonce to make
// every challenge's id unique. Payment credentials are removed from the request the
// resource sees. A challenge
// the host's method cannot fit in 8 KiB, or a request the host priced differently from
// what a method asked, is the host's error and answers 500.
//
// What it provides.
//
//   - Challenge, Credential and Receipt: the three wire objects, strict where the draft
//     says MUST and tolerant of unknown parameters and fields where it says so.
//   - Gate: net/http middleware for a resource server. The host supplies the price,
//     the caller, the secret and the Store; the Gate never chooses an amount.
//   - Client: an http.RoundTripper that answers a 402 through a Payer.
//   - Method: the plug-in a payment method implements. Stripe is the one implemented
//     (shared payment tokens, charge intent), and Commerce settles it through the
//     api/billing checkout.
//   - Info: the x-payment-info OpenAPI discovery extension.
//
// What it does not provide. The session and subscription intents, and the tempo, evm,
// solana, lightning, card and other method drafts, need a chain, a channel, a card
// network or a facilitator that this repository does not run, and are not implemented.
// Rate limiting (section 11.12) belongs to the host, and so does a durable, shared
// Store: Memory is for tests and a single process.
package mpp

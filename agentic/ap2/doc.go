// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

// Package ap2 is the merchant and verifier side of the Agent Payments Protocol (AP2 v0.2,
// github.com/google-agentic-commerce/AP2, docs/ap2).
//
// AP2 secures an agent's purchase with two mandates: a Checkout Mandate that proves the
// agent may buy the checkout the merchant signed, and a Payment Mandate that proves it may
// pay for it. Each is an SD-JWT. A user, or an agent provider trusted by the verifier, signs
// the mandate; or signs an open mandate of constraints and the agent signs the closed mandate
// under it with the key the open mandate names in cnf.
//
// This package builds and checks what a merchant and its payment processor handle:
//
//   - the mandates, constraints and receipts of the schemas (mandate.go, receipt.go);
//   - SD-JWT disclosures (RFC 9901), the delegation chain of an open and a closed token joined
//     by "~~", and ES256 compact JWS (sdjwt.go, verify.go, jws.go);
//   - the merchant's checks and the constraint evaluation the spec defines (verify.go,
//     constraint.go) and the dispute checks (dispute.go);
//   - the Seller, which signs a checkout and completes it against both mandates, the
//     checkout's cart agreeing with the store's before money moves (merchant.go), over a
//     Backend; Commerce is the Backend over commerce's checkout (commerce.go).
//
// What the spec defines and what it leaves to a sample or to another protocol:
//
//   - Defined by AP2: the mandate types and their schemas, the verification rules of each
//     role, the constraints and their evaluation, the receipts, the error codes, and the
//     hashes (sd_hash, checkout_hash, transaction_id, reference) over the compact
//     serialization of the tokens. AP2 defines no JSON canonicalization: every hash and
//     signature is over bytes as sent.
//   - Defined by the Delegate SD-JWT draft the spec cites: the chain serialization and the
//     key-bound closed token. The draft is not in the spec; its rules are those the AP2
//     examples show, which this package follows exactly.
//   - Not defined by AP2: how mandates travel. AP2 is "a security feature within a Commerce
//     Protocol", and the catalog and checkout APIs are outside its scope (UCP is the
//     compatible one; its AP2 extension places a mandate in `ap2.checkout_mandate`). There
//     is no HTTP or A2A wire here to mount: a host passes Seller the strings it received.
//   - Not implemented: the User Credential delegation model (OpenID4VP transaction_data,
//     whose key-binding is in the Delegate SD-JWT draft), a trailing KB-JWT, delegation
//     between agents, X.509 chains (x5c) for a first token's key, and mDoc credentials. The
//     host names the keys it trusts (Trust).
package ap2

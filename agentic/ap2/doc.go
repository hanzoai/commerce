// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

// Package ap2 is the merchant and verifier side of the Agent Payments Protocol (AP2 v0.2,
// github.com/google-agentic-commerce/AP2, docs/ap2): the Checkout and Payment mandates of its
// schemas as SD-JWTs (RFC 9901) in the delegation chain of the Delegate SD-JWT draft, their
// verification by the merchant and the merchant payment processor, the constraints and the receipts.
//
// A Seller signs a checkout and completes it against both mandates over a Backend; Commerce is the
// Backend over commerce's checkout. AP2 defines no wire for mandates (it is a security feature of a
// commerce protocol), so a host passes the Seller the strings it received. Every hash and signature
// is over bytes as sent: AP2 defines no JSON canonicalization.
//
// Not implemented: the User Credential model (OpenID4VP), a trailing KB-JWT, delegation between
// agents, x5c chains and mDoc. The host names the keys it trusts (Trust).
package ap2

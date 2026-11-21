// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

// Package mpp is the "Payment" HTTP authentication scheme (Machine Payments Protocol): a
// 402 challenge, an Authorization credential, a Payment-Receipt.
//
// It implements draft-httpauth-payment-01 and, where the editors' working copy
// (tempoxyz/mpp-specs 7bb99c85) differs, the working copy; the charge intent
// (draft-payment-intent-charge-00), the stripe method (draft-stripe-charge-00) and
// x-payment-info (draft-payment-discovery-01). These are Internet-Drafts, not standards.
//
// Not implemented: the session and subscription intents, and the tempo, evm, solana,
// lightning, card and other methods, which need a chain, channel or card network this
// repository does not run. Rate limiting (section 11.12) is the host's.
package mpp

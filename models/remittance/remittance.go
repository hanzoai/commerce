// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

// Package remittance is the durable record of ONE signed USDC payment a payer
// sent to Hanzo's treasury through Circle Gateway, and of what became of it.
//
// A record's storage id is a pure function of the payment's identity on the
// chain — (network, payer address, EIP-3009 nonce) — and it lives in the
// SYSTEM namespace, so the same authorization presented twice, by one org or by
// two, by one replica or by two, lands on one row. Gateway itself refuses a
// second use of a (payer address, nonce), so one row is one payment.
package remittance

import (
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/orm"
)

func init() {
	orm.Register[Remittance]("remittance", orm.WithStringKey[Remittance]())
}

// Where a remittance stands.
//
//	pending   recorded; Gateway's answer is not known yet (in flight, or a
//	          timeout). Nothing is credited. The reconciler finds out.
//	accepted  Gateway verified the signature and LOCKED the payer's balance.
//	          Credited to the payer's account here.
//	settled   the batch carrying it landed onchain. Final.
//	failed    Gateway failed it after accepting it, or it expired unaccepted.
//	          Never charged; any credit is reversed. Final.
//	refused   Gateway refused it outright (no balance, bad signature, …).
//	          Never charged, never credited. Presenting the same signed payment
//	          again tries again.
const (
	Pending  = "pending"
	Accepted = "accepted"
	Settled  = "settled"
	Failed   = "failed"
	Refused  = "refused"
)

// Remittance is one payment and its fate. There is deliberately no field named
// Key (it would shadow Model[T].Key(); see entity_guard.go).
type Remittance struct {
	mixin.Model[Remittance]

	// Who it credits: the ledger (Org), the account in it (Payer, the caller's
	// billing subject) and which books (Test). Resolved from the authenticated
	// caller when the payment was presented, never from the payment.
	Org   string `json:"org"`
	Payer string `json:"payer"`
	Test  bool   `json:"test"`

	// The payment as signed: CAIP-2 network, token, and the EIP-3009 fields,
	// addresses and nonce in lowercase 0x hex.
	Network     string `json:"network"`
	Asset       string `json:"asset"`
	Contract    string `json:"contract"` // the GatewayWallet the domain names
	From        string `json:"from"`
	To          string `json:"to"`
	Value       string `json:"value"` // base units
	ValidAfter  string `json:"validAfter"`
	ValidBefore string `json:"validBefore"`
	Nonce       string `json:"nonce"`
	Salt        string `json:"salt"`
	Signature   string `json:"signature" datastore:",noindex"`

	// AmountCents is Value in ledger cents: USDC is pegged 1:1 and has 6
	// decimals, and only whole cents are accepted, so this is exact.
	AmountCents int64 `json:"amountCents"`

	Status string `json:"status" orm:"default:pending"`
	// Reason is Gateway's errorReason for a refusal or failure, or ours.
	Reason string `json:"reason,omitempty"`
	// Payment is Gateway's id for it (GET /v1/x402/transfers/{id}).
	Payment string `json:"payment,omitempty"`
	// TxHash is the onchain settlement of the batch that carried it.
	TxHash string `json:"txHash,omitempty"`

	// Credited is set once the payer's account holds the money; Reversed once a
	// payment that failed after acceptance has had that credit taken back.
	Credited bool `json:"credited"`
	Reversed bool `json:"reversed"`
	// Sent is set before the first settle call goes out. A pending remittance
	// that was sent may have been accepted with nobody hearing the answer, so
	// it is looked up before it is sent again, and a refusal of the re-send is
	// not believed (billing/gateway drive).
	Sent bool `json:"sent"`

	SettledAt time.Time `json:"settledAt,omitempty"`
}

func (r *Remittance) Load(ps []datastore.Property) error  { return datastore.LoadStruct(r, ps) }
func (r *Remittance) Save() ([]datastore.Property, error) { return datastore.SaveStruct(r) }

// New returns a Remittance bound to db.
func New(db *datastore.Datastore) *Remittance {
	r := new(Remittance)
	r.Init(db)
	return r
}

// Query is a query over remittances in db's namespace.
func Query(db *datastore.Datastore) datastore.Query { return db.Query("remittance") }

// Open reports whether the remittance can still change: Gateway has not said,
// or it accepted and the batch has not landed.
func (r *Remittance) Open() bool { return r.Status == Pending || r.Status == Accepted }

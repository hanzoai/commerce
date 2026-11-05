// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

// Package remittance is the durable record of ONE signed USDC payment a payer
// made out to Hanzo's treasury — an EIP-3009 transferWithAuthorization the rail
// relays (billing/gateway) — and of what became of it.
//
// A record's storage id is a pure function of the payment's identity on the
// chain — (network, payer address, EIP-3009 nonce) — and it lives in the
// SYSTEM namespace, so the same authorization presented twice, by one org or by
// two, by one replica or by two, lands on one row. The USDC contract itself
// refuses a second use of a (payer address, nonce), so one row is one payment.
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
//	pending     recorded; the chain shows no use of the authorization yet. The
//	            relayer has sent it, or will. Nothing is credited.
//	confirming  the chain shows it used, in a transaction that paid the
//	            treasury, not yet Confirmations deep. Nothing is credited.
//	succeeded   that transaction is deep enough, and the payer is credited.
//	            Final.
//	refused     the token would refuse it now (no balance, a blocked address,
//	            …), so nothing was sent. Presenting the same signed payment
//	            again tries again, and the chain is still watched for its use
//	            until it expires.
//	failed      it can never pay the treasury: it expired unused, or its nonce
//	            was spent on another authorization. Never credited. Final.
const (
	Pending    = "pending"
	Confirming = "confirming"
	Succeeded  = "succeeded"
	Refused    = "refused"
	Failed     = "failed"
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
	// Reason is why it was refused or failed.
	Reason string `json:"reason,omitempty"`

	// The relayer's part. Sends is every transferWithAuthorization the relayer
	// signed for this payment, by hash, in order, each written BEFORE it was
	// broadcast: the record of what Hanzo's key did. SentAt is the last one.
	Relayer string    `json:"relayer,omitempty"`
	Sends   []string  `json:"sends,omitempty" datastore:",noindex"`
	SentAt  time.Time `json:"sentAt,omitempty"`
	// FromBlock is where a search of the chain for a use of this authorization
	// starts: the block the chain stood at when the payment was presented,
	// less the blocks since it became valid.
	FromBlock int64 `json:"fromBlock"`

	// The use on chain, once seen: the transaction that spent the
	// authorization, its block, and how deep it was when last read.
	TxHash        string `json:"txHash,omitempty"`
	Block         int64  `json:"block,omitempty"`
	Confirmations int64  `json:"confirmations,omitempty"`

	// The credit, in two steps that each happen once. Credited is written
	// BEFORE the ledger is asked: from then the account is owed this money,
	// and a landing that did not finish is finished by the reconciler. Landed
	// is written once the ledger and the receipt hold it.
	Credited bool `json:"credited"`
	Landed   bool `json:"landed"`

	// Due is when the reconciler next looks, as 20-digit zero-padded unix
	// seconds: a fixed width compares the same as text on every backend, so
	// the reconciler can ask for the rows that are due, oldest first. Tries
	// is how many looks in a row changed nothing; the wait doubles with it.
	Due   string `json:"due"`
	Tries int    `json:"tries"`

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

// Open reports whether the remittance can still change on its own: the chain
// may yet show it used, or it is used and not yet credited.
func (r *Remittance) Open() bool {
	return r.Status == Pending || r.Status == Confirming || r.Status == Refused
}

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
	orm.Register[Hold]("remittance-hold", orm.WithStringKey[Hold]())
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

	// State is Gateway's own word for the payment, as last read: received,
	// batched, confirmed, completed or failed. Only completed is final.
	State string `json:"state,omitempty"`

	// The credit, in three steps that each happen once. Credited is written
	// BEFORE the ledger is asked: the account is owed this money from then on,
	// so a payment Gateway fails later is always clawed back, even if the
	// process stopped between the ledger's answer and the next write. Landed
	// is written once the ledger and the receipt hold it. Reversed once a
	// payment Gateway failed has had it taken back.
	Credited bool `json:"credited"`
	Landed   bool `json:"landed"`
	Reversed bool `json:"reversed"`
	// Deferred is set while the credit waits for Gateway to confirm the
	// payment, because the payer already holds the most unconfirmed credit the
	// rail extends (billing/gateway Hold).
	Deferred bool `json:"deferred"`

	// Sent is set before the first settle call goes out. A pending remittance
	// that was sent may have been accepted with nobody hearing the answer, so
	// it is looked up before it is sent again, and a refusal of the re-send is
	// not believed (billing/gateway drive).
	Sent bool `json:"sent"`

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

// Open reports whether the remittance can still change: Gateway has not said,
// or it accepted and the batch has not landed.
func (r *Remittance) Open() bool { return r.Status == Pending || r.Status == Accepted }

// Hold is one payer's unconfirmed credit: the remittances credited on Gateway's
// acceptance whose payments Gateway has not yet confirmed onchain, and what
// each is worth. It is a SET keyed by remittance, so adding one twice holds it
// once and taking one out twice takes it out once, and every change to it is a
// swap over the row read, so two payments accepted at once for one payer
// cannot both see room that only one of them fits in.
type Hold struct {
	mixin.Model[Hold]

	Org   string           `json:"org"`
	Payer string           `json:"payer"`
	Test  bool             `json:"test"`
	Items map[string]int64 `json:"items"` // remittance id → cents
}

func (h *Hold) Load(ps []datastore.Property) error  { return datastore.LoadStruct(h, ps) }
func (h *Hold) Save() ([]datastore.Property, error) { return datastore.SaveStruct(h) }

// Total is the cents held.
func (h *Hold) Total() int64 {
	var n int64
	for _, c := range h.Items {
		n += c
	}
	return n
}

// NewHold returns a Hold bound to db.
func NewHold(db *datastore.Datastore) *Hold {
	h := new(Hold)
	h.Init(db)
	return h
}

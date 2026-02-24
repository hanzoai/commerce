package cryptopaymentintent

import (
	"time"

	"github.com/hanzoai/commerce/datastore"
	. "github.com/hanzoai/commerce/types"
)

var kind = "crypto-payment-intent"

func (cpi CryptoPaymentIntent) Kind() string {
	return kind
}

func (cpi *CryptoPaymentIntent) Init(db *datastore.Datastore) {
	cpi.Model.Init(db, cpi)
}

func (cpi *CryptoPaymentIntent) Defaults() {
	cpi.Parent = cpi.Db.NewKey("synckey", "", 1, nil)
	if cpi.Status == "" {
		cpi.Status = Pending
	}
	if cpi.Currency == "" {
		cpi.Currency = "usd"
	}
	if cpi.SettlementCurrency == "" {
		cpi.SettlementCurrency = "usd"
	}
	if cpi.ExpiresAt.IsZero() {
		cpi.ExpiresAt = time.Now().Add(30 * time.Minute)
	}
	if cpi.RequiredConfirmations == 0 && cpi.Chain != "" {
		cpi.RequiredConfirmations = RequiredConfirmationsForChain(cpi.Chain)
	}
	if cpi.Metadata == nil {
		cpi.Metadata = make(Map)
	}
}

func New(db *datastore.Datastore) *CryptoPaymentIntent {
	cpi := new(CryptoPaymentIntent)
	cpi.Init(db)
	cpi.Defaults()
	return cpi
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}

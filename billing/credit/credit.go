// Package credit provides reusable starter credit logic.
// Shared between api/billing handlers and middleware/iammiddleware.
package credit

import (
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/transaction"
	"github.com/hanzoai/commerce/models/types/currency"

	. "github.com/hanzoai/commerce/types"
)

// Starter credit constants.
const (
	StarterCreditCents = 500 // $5.00 USD
	StarterCreditDays  = 30  // expires in 30 days
	StarterCreditTag   = "starter-credit"
)

// GrantIfEligible checks if the user already has a starter credit and
// grants one if not. It is idempotent: duplicate calls for the same
// user are safe. The trigger parameter records what initiated the grant
// (e.g. "payment-method-added", "org-created").
//
// The check-and-create runs inside a transaction to prevent race conditions
// where concurrent goroutines could both pass the existence check and
// grant duplicate credits.
//
// This function is intended to be called from a goroutine; it only logs
// on failure and never panics.
func GrantIfEligible(db *datastore.Datastore, userId, trigger string) {
	if userId == "" {
		return
	}

	err := db.RunInTransaction(func(txDb *datastore.Datastore) error {
		rootKey := txDb.NewKey("synckey", "", 1, nil)

		// Check if starter credit was already granted (inside transaction).
		existingTrans := make([]*transaction.Transaction, 0)
		tq := transaction.Query(txDb).Ancestor(rootKey).
			Filter("DestinationId=", userId).
			Filter("Tags=", StarterCreditTag)
		if _, err := tq.Limit(1).GetAll(&existingTrans); err == nil && len(existingTrans) > 0 {
			return nil // already granted
		}

		trans := transaction.New(txDb)
		trans.Type = transaction.Deposit
		trans.DestinationId = userId
		trans.DestinationKind = "iam-user"
		trans.Currency = "usd"
		trans.Amount = currency.Cents(StarterCreditCents)
		trans.Notes = "Welcome credit: $5.00 USD (expires in 30 days)"
		trans.Tags = StarterCreditTag
		trans.ExpiresAt = time.Now().AddDate(0, 0, StarterCreditDays)
		trans.Metadata = Map{
			"creditType": "starter",
			"expiryDays": StarterCreditDays,
			"trigger":    trigger,
		}

		return trans.Create()
	}, nil)

	if err != nil {
		log.Warn("Failed to auto-grant starter credit for user %s: %v", userId, err)
	}
}

package tasks

import (
	"appengine"

	"github.com/gin-gonic/gin"
	stripe "github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/client"
	"github.com/stripe/stripe-go/dispute"

	"crowdstart.io/config"
	"crowdstart.io/datastore"
	"crowdstart.io/datastore/parallel"
	"crowdstart.io/models"
	. "crowdstart.io/thirdparty/stripe"
	"crowdstart.io/util/log"
)

// This is a worker that processes one order at a time
var synchronizeCharges = parallel.Task("synchronize-charges", SynchronizeCharge)

func SynchronizeCharge(db *datastore.Datastore, key datastore.Key, o models.Order, sc *client.API) error {
	description := o.Description()
	for i, charge := range o.Charges {
		updatedCharge, err := sc.Charges.Get(charge.ID, nil)
		if err != nil {
			log.Error("Failed to get charges for %v: %v", charge.ID, err)
			return err
		}

		if updatedCharge.Desc != description {
			params := &stripe.ChargeParams{Desc: description}
			var err error
			updatedCharge, err = sc.Charges.Update(charge.ID, params)
			if err != nil {
				return err
			}
		}
		o.Charges[i] = models.Charge{
			ID:             updatedCharge.ID,
			Captured:       updatedCharge.Captured,
			Created:        updatedCharge.Created,
			Desc:           updatedCharge.Desc,
			Email:          updatedCharge.Email,
			FailCode:       updatedCharge.FailCode,
			FailMsg:        updatedCharge.FailMsg,
			Live:           updatedCharge.Live,
			Paid:           updatedCharge.Paid,
			Refunded:       updatedCharge.Refunded,
			Statement:      updatedCharge.Statement,
			Amount:         int64(updatedCharge.Amount), // TODO: Check if this is necessary.
			AmountRefunded: int64(updatedCharge.AmountRefunded),
		}

		if updatedCharge.Dispute != nil {
			// TODO: Refactor for multiple charges.
			// o.Dispute = stripeWrapperModels.ConvertDispute(*updatedCharge.Dispute)
			o.Disputed = true
			if updatedCharge.Dispute.Status != dispute.Won {
				o.Locked = true
			}
		}

		if charge.Refunded {
			o.Refunded = true
		}
	}

	log.Info("Refunded: %v", o.Refunded)
	if _, err := db.PutKind("order", key, &o); err != nil {
		return err
	}
	return nil
}

func RunSynchronizeCharges(c *gin.Context) {
	ctx := c.MustGet("appengine").(appengine.Context)
	sc := NewApiClient(ctx, config.Stripe.TestSecretKey)
	parallel.Run(c, "order", 100, synchronizeCharges, sc)
}

func init() {
}

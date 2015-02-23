package tasks

import (
	"github.com/gin-gonic/gin"
	stripe "github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/dispute"

	"crowdstart.io/config"
	"crowdstart.io/datastore"
	"crowdstart.io/datastore/parallel"
	"crowdstart.io/models"
	. "crowdstart.io/thirdparty/stripe"
	"crowdstart.io/util/log"
	"crowdstart.io/util/task"
)

var synchronizeCharges = parallel.Task("synchronize-charges", func(db *datastore.Datastore, key datastore.Key, o models.Order, accessToken string) error {
	log.Info("Synchronising")
	sc := NewApiClient(db.Context, accessToken)

	description := o.Description()
	for i, charge := range o.Charges {
		updatedCharge, err := sc.Charges.Get(charge.ID, nil)
		if err != nil {
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
			o.Dispute = *updatedCharge.Dispute // TODO: Refactor for multiple charges.
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
})

func SynchronizeCharges(c *gin.Context) {
	parallel.Run(c, "order", 100, synchronizeCharges, config.Stripe.APISecret)
}

func init() {
	task.Register("stripe-sync-orders", SynchronizeCharges)
}

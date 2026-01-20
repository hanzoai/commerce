package tasks

import (
	"context"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/util/cache"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/fee"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/payment"
	"github.com/hanzoai/commerce/models/transfer"
	"github.com/hanzoai/commerce/thirdparty/stripe"
	"github.com/hanzoai/commerce/util/json"
)

// Get namespaced context for given namespace
func getNamespacedContext(ctx context.Context, ns string) context.Context {
	log.Debug("Setting namespace of context to %s", ns, ctx)
	// Note: namespace handling removed - implement alternative if needed
	return ctx
}

// Grab organization out of cache
func getOrganization(ctx context.Context) *organization.Organization {
	org := &organization.Organization{}
	item, err := cache.Get(ctx, "organization")
	if err != nil {
		log.Error("Failed to get organization from cache: %v", err, ctx)
		return org
	}

	// Decode organization
	err = json.DecodeBytes(item.Value, org)
	if err != nil {
		log.Error("Failed to decode organization: %v", err, ctx)
	}
	return org
}

// Get ancestor for ancestor query for a payment associated with a stripe charge
func getPaymentFromCharge(ctx context.Context, ch *stripe.Charge) (*payment.Payment, bool, error) {
	db := datastore.New(ctx)
	pay := payment.New(db)

	id, ok := ch.Metadata["payment"]

	// Try to get by payment id
	if ok {
		log.Debug("Try to get payment by payment id: %v", id, ctx)
		if err := pay.GetById(id); err == nil {
			return pay, true, nil
		}
	}

	// Try to lookup payment using charge id
	log.Debug("Lookup payment by charge id: %v", ch.ID, ctx)
	ok, err := pay.Query().Filter("Account.ChargeId=", ch.ID).Get()
	return pay, ok, err
}

// Get our transfer from a stripe transfer
func getTransfer(ctx context.Context, str *stripe.Transfer) (*transfer.Transfer, bool, error) {
	db := datastore.New(ctx)
	tr := transfer.New(db)

	id, ok := str.Metadata["transfer"]

	// Try to get by transfer id
	if ok {
		log.Debug("Try to get transfer by transfer id: %v", id, ctx)
		if err := tr.GetById(id); err == nil {
			return tr, true, nil
		}
	}

	// Try to lookup transfer using transfer id
	log.Debug("Lookup transfer by transfer id: %v", str.ID, ctx)
	ok, err := tr.Query().Filter("Account.TransferId=", str.ID).Get()
	return tr, ok, err
}

// Update charge in case order/pay id is missing in metadata
func updateChargeFromPayment(ctx context.Context, token string, pay *payment.Payment, ch *stripe.Charge) {
	if ch != nil {
		// Check if we need to sync back changes to charge
		payId, _ := ch.Metadata["payment"]
		ordId, _ := ch.Metadata["order"]
		usrId, _ := ch.Metadata["user"]

		// Don't sync if metadata is already correct
		if pay.Id() == payId && pay.OrderId == ordId && pay.Buyer.UserId == usrId {
			return
		}
	}

	// Get a stripe client
	client := stripe.New(ctx, token)

	// Update charge with new metadata
	if _, err := client.UpdateCharge(pay); err != nil {
		log.Error("Unable to update charge for payment '%s': %v", pay.Id(), err, ctx)
	}
}

func UpdateFeesFromPayment(fees []*fee.Fee, pay *payment.Payment) {
	var feeStatus fee.Status

	switch pay.Status {
	case payment.Paid:
		feeStatus = fee.Payable
	case payment.Refunded:
		feeStatus = fee.Refunded
	case payment.Disputed:
		feeStatus = fee.Disputed
	case payment.Unpaid:
		feeStatus = fee.Pending
	default:
		log.Warn("Unhandled payment state: '%s'", pay.Status, pay.Db.Context)
	}

	for _, fe := range fees {
		// Ignore transferred fees
		if fe.Status == fee.Transferred {
			continue
		}

		fe.Status = feeStatus
		fe.MustUpdate()
	}
}

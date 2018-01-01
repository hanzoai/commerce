package checkout

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/api/checkout/balance"
	"hanzo.io/api/checkout/null"
	"hanzo.io/api/checkout/stripe"
	"hanzo.io/api/checkout/tasks"
	"hanzo.io/api/checkout/util"
	"hanzo.io/models/order"
	"hanzo.io/models/organization"
	"hanzo.io/models/payment"
	"hanzo.io/util/webhook"
)

// Make the context less ambiguous, saveReferral needs org context for example
func capture(c *gin.Context, org *organization.Organization, ord *order.Order) error {
	var err error
	var payments []*payment.Payment

	switch ord.Type {
	case "null":
		ord, payments, err = null.Capture(org, ord)
	case "balance":
		ord, payments, err = balance.Capture(org, ord)
	case "stripe":
		ord, payments, err = stripe.Capture(org, ord)
	case "paypal":
		payments = ord.Payments
	default:
		// TODO: return nil, errors.New("Invalid order type")
		ord, payments, err = stripe.Capture(org, ord)
	}

	if err != nil {
		return err
	}

	ctx := ord.Context()

	util.UpdateOrder(ctx, ord, payments)

	if err := util.UpdateOrderPayments(ctx, ord, payments); err != nil {
		return err
	}

	// TODO: Run in task(CaptureAsync), no need to block call on rest of this
	util.SaveRedemptions(ctx, ord)
	util.UpdateReferral(org, ord)
	util.UpdateCart(ctx, ord)
	util.UpdateStats(ctx, org, ord, payments)
	util.HandleDeposit(ord)

	buyer := payments[0].Buyer

	tasks.CaptureAsync.Call(org.Context(), org.Id(), ord.Id())
	tasks.SendOrderConfirmation.Call(org.Context(), org.Id(), ord.Id(), buyer.Email, buyer.FirstName, buyer.LastName)

	webhook.Emit(ctx, org.Name, "order.paid", ord)
	return nil
}

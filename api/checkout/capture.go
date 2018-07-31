package checkout

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/email"
	"hanzo.io/api/checkout/authorizenet"
	"hanzo.io/api/checkout/balance"
	"hanzo.io/api/checkout/null"
	"hanzo.io/api/checkout/stripe"
	"hanzo.io/api/checkout/tasks"
	"hanzo.io/api/checkout/util"
	"hanzo.io/models/order"
	"hanzo.io/models/organization"
	"hanzo.io/models/payment"
	"hanzo.io/models/types/accounts"
	"hanzo.io/models/user"
	"hanzo.io/util/webhook"

	"hanzo.io/log"
)

// Make the context less ambiguous, saveReferral needs org context for example
func capture(c *gin.Context, org *organization.Organization, ord *order.Order) error {
	var err error
	var payments []*payment.Payment

	switch ord.Type {
	case accounts.AuthorizeNetType:
		ord, payments, err = authorizenet.Capture(org, ord)
	case accounts.BalanceType:
		ord, payments, err = balance.Capture(org, ord)
	case accounts.NullType:
		ord, payments, err = null.Capture(org, ord)
	case accounts.StripeType:
		ord, payments, err = stripe.Capture(org, ord)
	case accounts.PayPalType:
		payments = ord.Payments
	default:
		// TODO: return nil, errors.New("Invalid order type")
		ord, payments, err = stripe.Capture(org, ord)
	}

	if err != nil {
		log.Error("Capture failed: %v", err, c)
		return err
	}

	ctx := ord.Context()

	util.UpdateOrder(ctx, ord, payments)

	if err := util.UpdateOrderPayments(ctx, ord, payments); err != nil {
		log.Error("Capture could not update order/payments: %v", err, c)
		return err
	}

	// TODO: Run in task(CaptureAsync), no need to block call on rest of this
	util.SaveRedemptions(ctx, ord)
	util.UpdateReferral(org, ord)
	util.UpdateCart(ctx, ord)
	util.UpdateStats(ctx, org, ord, payments)
	util.HandleDeposit(ord)

	tasks.CaptureAsync.Call(org.Context(), org.Id(), ord.Id())

	usr := user.New(ord.Db)
	err = usr.GetById(ord.UserId)
	if err != nil {
		log.Error("Capture could not find User %v: %v", usr.Id(), err, c)
		return err
	}

	email.SendOrderConfirmation(org.Context(), org, ord, usr)

	webhook.Emit(ctx, org.Name, "order.paid", ord)
	return nil
}

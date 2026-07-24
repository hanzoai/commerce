package checkout

import (
	"context"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/api/checkout/authorizenet"
	"github.com/hanzoai/commerce/api/checkout/balance"
	"github.com/hanzoai/commerce/api/checkout/null"
	"github.com/hanzoai/commerce/api/checkout/square"
	"github.com/hanzoai/commerce/api/checkout/tasks"
	"github.com/hanzoai/commerce/api/checkout/util"
	"github.com/hanzoai/commerce/billing/engine"
	"github.com/hanzoai/commerce/email"
	"github.com/hanzoai/commerce/events"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/order"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/payment"
	"github.com/hanzoai/commerce/models/types/accounts"
	"github.com/hanzoai/commerce/models/user"
	"github.com/hanzoai/commerce/util/webhook"
)

// Make the context less ambiguous, saveReferral needs org context for example
func capture(c *zip.Ctx, org *organization.Organization, ord *order.Order) error {
	var err error
	var payments []*payment.Payment

	switch ord.Type {
	case accounts.AuthorizeNetType:
		ord, payments, err = authorizenet.Capture(org, ord)
	case accounts.BalanceType:
		ord, payments, err = balance.Capture(org, ord)
	case accounts.NullType:
		ord, payments, err = null.Capture(org, ord)
	case accounts.SquareType:
		ord, payments, err = square.Capture(org, ord)
	case accounts.PayPalType:
		payments = ord.Payments
	default:
		ord, payments, err = square.Capture(org, ord)
	}

	if err != nil {
		ord.CancelReservations()

		log.Error("Capture failed: %v", err, c)
		return err
	}

	ctx := ord.Context()

	util.UpdateOrder(ctx, ord, payments)

	if err := util.UpdateOrderPayments(ctx, ord, payments); err != nil {
		ord.CancelReservations()

		log.Error("Capture could not update order/payments: %v", err, c)
		return err
	}

	// TODO: Run in task(CaptureAsync), no need to block call on rest of this
	util.SaveRedemptions(ctx, ord)
	util.UpdateReferral(org, ord)
	util.UpdateCart(ctx, ord)
	util.UpdateStats(ctx, org, ord, payments)
	util.UpdateWoopraIntegration(ctx, org, ord)
	util.HandleDeposit(ord)

	tasks.CaptureAsync.Call(org.Context(), org.Id(), ord.Id())

	usr := user.New(ord.Datastore())
	err = usr.GetById(ord.UserId)
	if err != nil {
		log.Error("Capture could not find User %v: %v", usr.Id(), err, c)
		return err
	}

	email.SendOrderConfirmation(org.Context(), org, ord, usr)

	webhook.Emit(ctx, org.Name, "order.paid", ord)

	// Publish order.completed to NATS/JetStream with GA4 + Facebook CAPI (fire and forget)
	if pub := c.Locals("publisher"); pub != nil {
		if p, ok := pub.(*events.Publisher); ok {
			orderItems := orderLineItemInfos(ord)
			// Detach from the request (survive client disconnect) but keep trace
			// values — WithoutCancel, not Background. Captured before the goroutine.
			ctx := context.WithoutCancel(c.Context())
			go func() {
				// The request ctx is pooled/released after the handler returns,
				// so the fire-and-forget goroutine must not reference c.
				if pubErr := p.PublishOrderCompleted(ctx, ord.Id(), org.Name, usr.Id(), usr.Email,
					int64(ord.Total), string(ord.Currency), events.ToOrderItems(orderItems)); pubErr != nil {
					log.Error("PublishOrderCompleted: %v", pubErr)
				}
			}()
		}
	}

	// Deliver order.completed to subscribed billing webhook endpoints (fire and
	// forget, off the money path — same lifecycle point as the bus publisher).
	// Detach the request context (zip recycles c after the handler) before the
	// goroutine, then namespace it to the org.
	engine.Emit(org.Namespaced(context.WithoutCancel(c.Context())), "order.completed", "order", ord.Id(), usr.Id(), map[string]interface{}{
		"orderId":  ord.Id(),
		"revenue":  float64(ord.Total) / 100.0,
		"currency": string(ord.Currency),
		"email":    usr.Email,
	})

	return nil
}

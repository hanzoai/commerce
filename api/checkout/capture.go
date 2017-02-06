package checkout

import (
	"appengine"

	"github.com/gin-gonic/gin"

	"crowdstart.com/api/checkout/balance"
	"crowdstart.com/api/checkout/null"
	"crowdstart.com/api/checkout/stripe"
	"crowdstart.com/api/checkout/tasks"
	"crowdstart.com/models/multi"
	"crowdstart.com/models/order"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/payment"
	"crowdstart.com/models/types/currency"
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

	updateOrder(ctx, ord, payments)
	if err := saveOrder(ctx, ord, payments); err != nil {
		return err
	}

	tasks.CaptureAsync.Call(org.Context(), org.Id(), ord.Id())
	return nil
}

func updateOrder(ctx appengine.Context, ord *order.Order, payments []*payment.Payment) {
	totalPaid := 0

	for _, pay := range payments {
		totalPaid += int(pay.Amount)
	}

	ord.Paid = currency.Cents(int(ord.Paid) + totalPaid)
	if ord.Paid == ord.Total {
		ord.PaymentStatus = payment.Paid
	}
}

func saveOrder(ctx appengine.Context, ord *order.Order, payments []*payment.Payment) error {
	vals := []interface{}{ord}

	for _, pay := range payments {
		vals = append(vals, pay)
	}

	return multi.Update(vals)
}

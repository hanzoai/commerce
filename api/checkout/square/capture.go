package square

import (
	"context"
	"errors"

	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/order"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/payment"
	squarelib "github.com/hanzoai/commerce/thirdparty/square"
)

var FailedToCapturePayment = errors.New("failed to capture Square payment")

func Capture(org *organization.Organization, ord *order.Order) (*order.Order, []*payment.Payment, error) {
	db := ord.Datastore()

	payments := make([]*payment.Payment, 0)
	if err := payment.Query(db).Ancestor(ord.Key()).GetModels(&payments); err != nil {
		return nil, payments, err
	}

	sqCfg := org.SquareConfig(!org.Live)
	proc := squarelib.NewProcessor(squarelib.Config{
		AccessToken:   sqCfg.AccessToken,
		LocationID:    sqCfg.LocationId,
		WebhookSecret: org.Square.WebhookSignatureKey,
		Environment:   squareEnv(org.Live),
	})

	for _, p := range payments {
		if !p.Captured {
			result, err := proc.Capture(context.Background(), p.Account.Square.PaymentId, p.Amount)
			if err != nil {
				return nil, payments, err
			}

			if result.Status != "captured" && !result.Success {
				return nil, payments, FailedToCapturePayment
			}

			p.Captured = true
			p.Status = payment.Paid
			p.AmountTransferred = p.Amount
			p.CurrencyTransferred = p.Currency
			if result.Fee > 0 {
				p.Fee = result.Fee
			}

			log.Info("Square captured payment '%s' → '%s'", p.Id(), result.TransactionID)
		}
	}

	return ord, payments, nil
}

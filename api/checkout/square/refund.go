package square

import (
	"context"
	"errors"

	"github.com/hanzoai/commerce/email"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/order"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/payment"
	"github.com/hanzoai/commerce/models/types/accounts"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/models/user"
	squarelib "github.com/hanzoai/commerce/thirdparty/square"
	"github.com/hanzoai/commerce/payment/processor"
)

var NonSquarePayment = errors.New("only refunds for Square payments are supported via this handler")
var ZeroRefund = errors.New("refund amount cannot be 0")
var NegativeRefund = errors.New("refund amount must be a positive integer")

func Refund(org *organization.Organization, ord *order.Order, refundAmount currency.Cents) error {
	if refundAmount == 0 {
		return ZeroRefund
	}
	if refundAmount < 0 {
		return NegativeRefund
	}

	db := ord.Db
	ctx := db.Context

	if int64(refundAmount) > int64(ord.Total) {
		return errors.New("requested refund amount is greater than the order total")
	}
	if ord.Refunded+refundAmount > ord.Total {
		return errors.New("previously refunded amounts and requested refund amount exceed the order total")
	}

	payments, err := ord.GetPayments()
	if err != nil {
		return err
	}

	for _, pay := range payments {
		if pay.Type != accounts.SquareType {
			return NonSquarePayment
		}
	}

	if ord.Paid < refundAmount {
		return errors.New("refund amount exceeds total payment amount")
	}

	sqCfg := org.SquareConfig(!org.Live)
	proc := squarelib.NewProcessor(squarelib.Config{
		AccessToken:   sqCfg.AccessToken,
		LocationID:    sqCfg.LocationId,
		WebhookSecret: org.Square.WebhookSignatureKey,
		Environment:   squareEnv(org.Live),
	})

	refundRemaining := refundAmount
	for _, p := range payments {
		var amount currency.Cents
		if p.Amount <= refundRemaining {
			amount = p.Amount
		} else {
			amount = refundRemaining
		}

		if !p.Test {
			_, err := proc.Refund(context.Background(), processor.RefundRequest{
				TransactionID: p.Account.Square.PaymentId,
				Amount:        amount,
				Reason:        "customer refund",
			})
			if err != nil {
				return err
			}
		}

		refundRemaining -= amount
		if refundRemaining == 0 {
			break
		}
	}

	log.Info("Square refund amount: %v", refundAmount)
	ord.Refunded = ord.Refunded + refundAmount
	ord.Paid = ord.Paid - refundAmount

	usr := user.New(db)
	usr.GetById(ord.UserId)

	if ord.Total == ord.Refunded {
		email.SendOrderRefunded(ctx, org, ord, usr, payments[0])
		ord.PaymentStatus = payment.Refunded
		ord.Status = order.Cancelled
	} else {
		email.SendOrderRefunded(ctx, org, ord, usr, payments[0])
	}

	return ord.Put()
}

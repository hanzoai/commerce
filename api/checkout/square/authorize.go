package square

import (
	"context"

	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/order"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/payment"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/models/user"
	squarelib "github.com/hanzoai/commerce/thirdparty/square"
	"github.com/hanzoai/commerce/payment/processor"
)

func Authorize(org *organization.Organization, ord *order.Order, usr *user.User, pay *payment.Payment) error {
	sqCfg := org.SquareConfig(!org.Live)

	proc := squarelib.NewProcessor(squarelib.Config{
		AccessToken:   sqCfg.AccessToken,
		LocationID:    sqCfg.LocationId,
		WebhookSecret: org.Square.WebhookSignatureKey,
		Environment:   squareEnv(org.Live),
	})

	req := processor.PaymentRequest{
		Amount:      currency.Cents(pay.Amount),
		Currency:    pay.Currency,
		Token:       pay.Account.Number, // Card nonce from client
		CustomerID:  usr.Accounts.Square.CustomerId,
		OrderID:     ord.Id(),
		Description: pay.Description,
	}

	result, err := proc.Authorize(context.Background(), req)
	if err != nil {
		log.Warn("Square authorize failed for payment '%s': %v", pay.Id(), err)
		return err
	}

	// Update payment with Square response
	pay.Account.Square.PaymentId = result.TransactionID
	pay.Account.Square.LocationId = sqCfg.LocationId
	pay.Live = org.Live

	// Update user's Square account
	usr.Accounts.Square.CustomerId = req.CustomerID

	log.Info("Square authorized payment '%s' → transaction '%s'", pay.Id(), result.TransactionID)
	return nil
}

func squareEnv(live bool) string {
	if live {
		return "production"
	}
	return "sandbox"
}

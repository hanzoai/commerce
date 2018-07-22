package authorizenet

import (
	"hanzo.io/log"
	"hanzo.io/models/order"
	"hanzo.io/models/organization"
	"hanzo.io/models/payment"
	"hanzo.io/models/user"
	"hanzo.io/thirdparty/authorizenet"
)

func Authorize(org *organization.Organization, ord *order.Order, usr *user.User, pay *payment.Payment) error {
	ctx := ord.Db.Context

	// Create stripe client
	con := org.AuthorizeNetTokens()

	loginId := con.LoginId
	transactionKey := con.TransactionKey
	key := con.Key

	pay.Amount = ord.Total

	client := authorizenet.New(ctx, loginId, transactionKey, key, false)

	// Do authorization
	_, err := client.Authorize(pay)
	if err != nil {
		log.Error("Failed to authorize payment '%s'", pay.Id(), ctx)
		log.JSON(pay)
		return err
	}

	usr.Accounts.AuthorizeNet = pay.Account.AuthorizeNet
	pay.Live = org.Live

	return nil
}

package util

import (
	"hanzo.io/datastore"
	"hanzo.io/models/order"
	"hanzo.io/models/user"
	"hanzo.io/models/wallet"
	"hanzo.io/util/log"
)

func GetUserOrderByWallet(db *datastore.Datastore, walletId string) (*user.User, *order.Order, *wallet.Wallet, error) {
	ctx := db.Context

	w := wallet.New(db)
	if err := w.GetById(walletId); err != nil {
		log.Warn("Could not find Wallet '%s': %v", walletId, err, ctx)
		return nil, nil, nil, err
	}

	// Check if there's an order with this wallet
	ord := order.New(db)
	if ok, err := ord.Query().Filter("WalletId=", w.Id()).Get(); !ok {
		if err != nil {
			log.Warn("No order found for Wallet '%s': %v", w.Id(), err, ctx)
			return nil, nil, nil, err
		}

		log.Warn("No order found for Wallet '%s'", w.Id(), ctx)
		return nil, nil, nil, nil
	}

	// Get user so we can get a buyer
	usr := user.New(db)
	if err := usr.GetById(ord.UserId); err != nil {
		log.Warn("User not found for Order '%s', Wallet '%s'", ord.Id(), w.Id(), ctx)
		return nil, nil, nil, err
	}

	return usr, ord, w, nil
}

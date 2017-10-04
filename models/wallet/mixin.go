package wallet

import (
	"hanzo.io/datastore"
)

type WalletHolder struct {
	WalletId string `json:"walletId"`
	Wallet   Wallet `json:"wallet,omitempty" datastore:"-"`
}

func (w *WalletHolder) GetOrCreateWallet(db *datastore.Datastore) (*Wallet, error) {
	wal := New(db)

	// create
	if w.WalletId == "" {
		if err := wal.Create(); err != nil {
			return nil, err
		}

		w.WalletId = wal.Id()
		w.Wallet = *wal
		return wal, nil
	}

	// get
	err := wal.GetById(w.WalletId)
	if wal != nil {
		w.Wallet = *wal
	}
	return wal, err
}

func (w *WalletHolder) LoadWallet(db *datastore.Datastore) error {
	wal, err := w.GetOrCreateWallet(db)

	if err != nil {
		return err
	}
	w.Wallet = *wal

	return nil
}

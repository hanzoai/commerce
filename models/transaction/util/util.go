package util

import (
	"context"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/transaction"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/json"
)

type TransactionData struct {
	Balance      currency.Cents             `json:"balance"`
	Holds        currency.Cents             `json:"holds"`
	Transactions []*transaction.Transaction `json:"transactions"`
}

type TransactionDatas struct {
	Id   string                             `json:"id"`
	Kind string                             `json:"kind"`
	Data map[currency.Type]*TransactionData `json:"data"`
}

func GetTransactions(ctx context.Context, id, kind string, test bool) (*TransactionDatas, error) {
	db := datastore.New(ctx)

	rootKey := db.NewKey("synckey", "", 1, nil)

	transs := make([]*transaction.Transaction, 0)
	if _, err := transaction.Query(db).Ancestor(rootKey).Filter("Test=", test).Filter("SourceKind=", kind).Filter("SourceId=", id).GetAll(&transs); err != nil {
		log.Error("ListSource Transaction Query Error '%v'", err, ctx)
		return nil, err
	}

	if _, err := transaction.Query(db).Ancestor(rootKey).Filter("Test=", test).Filter("DestinationKind=", kind).Filter("DestinationId=", id).GetAll(&transs); err != nil {
		log.Error("ListDestination Transaction Query Error '%v'", err, ctx)
		return nil, err
	}

	log.Info("GetTransactions (Test:%v) '%v/%v': %v", test, kind, id, json.Encode(transs), ctx)

	return TallyTransactions(ctx, id, kind, transs)
}

func GetTransactionsByCurrency(ctx context.Context, id, kind string, cur currency.Type, test bool) (*TransactionDatas, error) {
	db := datastore.New(ctx)

	rootKey := db.NewKey("synckey", "", 1, nil)

	transs := make([]*transaction.Transaction, 0)
	if _, err := transaction.Query(db).Ancestor(rootKey).Filter("Test=", test).Filter("SourceKind=", kind).Filter("SourceId=", id).Filter("Currency=", cur).GetAll(&transs); err != nil {
		log.Error("ListSource Transaction Query Error '%v'", err, ctx)
		return nil, err
	}

	if _, err := transaction.Query(db).Ancestor(rootKey).Filter("Test=", test).Filter("DestinationKind=", kind).Filter("DestinationId=", id).Filter("Currency=", cur).GetAll(&transs); err != nil {
		log.Error("ListDestination Transaction Query Error '%v'", err, ctx)
		return nil, err
	}

	log.Info("GetTransactionsByCurrency (Test:%v) '%v/%v', '%v': %v", test, kind, id, cur, json.Encode(transs), ctx)

	return TallyTransactions(ctx, id, kind, transs)
}

func TallyTransactions(ctx context.Context, id, kind string, transs []*transaction.Transaction) (*TransactionDatas, error) {
	datas := &TransactionDatas{
		Id:   id,
		Kind: kind,
		Data: make(map[currency.Type]*TransactionData),
	}

	now := time.Now()

	for _, trans := range transs {
		if trans.SourceId == trans.DestinationId {
			log.Warn("Anomylous transaction to self detected: '%v", trans.Id(), ctx)
			continue
		}

		// Skip expired deposits — they no longer contribute to balance.
		if trans.Type == transaction.Deposit && !trans.ExpiresAt.IsZero() && trans.ExpiresAt.Before(now) {
			log.Info("Skipping expired deposit %v (expired %v)", trans.Id(), trans.ExpiresAt, ctx)
			continue
		}

		if _, ok := datas.Data[trans.Currency]; !ok {
			datas.Data[trans.Currency] = &TransactionData{
				Transactions: make([]*transaction.Transaction, 0),
			}
		}

		data := datas.Data[trans.Currency]
		data.Transactions = append(data.Transactions, trans)

		switch trans.Type {
		case transaction.Deposit:
			data.Balance += trans.Amount
		case transaction.Withdraw:
			data.Balance -= trans.Amount
		case transaction.Transfer:
			if trans.DestinationId == id {
				data.Balance += trans.Amount
			} else if trans.SourceId == id {
				data.Balance -= trans.Amount
			} else {
				log.Panic("This should not be possible", ctx)
				return nil, nil
			}
		case transaction.Hold:
			data.Holds += trans.Amount
		case transaction.HoldRemoved:
		default:
			log.Panic("This should not be possible: '%v'", json.Encode(trans), ctx)
		}
	}

	for k, v := range datas.Data {
		if v.Holds < currency.Cents(0) {
			datas.Data[k].Holds = currency.Cents(0)
		}
	}

	return datas, nil
}

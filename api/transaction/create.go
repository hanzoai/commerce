package transaction

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/middleware"
	"hanzo.io/models/transaction"
	"hanzo.io/models/transaction/util"
	"hanzo.io/models/types/currency"
	"hanzo.io/util/json"
	"hanzo.io/util/json/http"
	"hanzo.io/log"
)

func Create(c *context.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))
	trans := transaction.New(db)

	// Decode response body to create new transaction
	if err := json.Decode(c.Request.Body, trans); err != nil {
		http.Fail(c, 400, err.Error(), err)
		return
	}

	if trans.Id_ != "" {
		log.Warn("Id_ should not be set, set to '%v', setting to ''", trans.Id_, c)
		trans.Id_ = ""
	}

	if trans.Type == transaction.Hold || trans.Type == transaction.HoldRemoved {
		log.Error("Transaction type should not be a hold: '%v'", trans.Type, c)
		http.Fail(c, 500, ErrorUseHoldApi.Error(), ErrorUseHoldApi)
		return
	}

	if trans.Type != transaction.Deposit && trans.Type != transaction.Withdraw && trans.Type != transaction.Transfer {
		log.Error("Transaction type is invalid: '%v'", trans.Type, c)
		http.Fail(c, 500, ErrorInvalidType.Error(), ErrorInvalidType)
		return
	}

	if trans.SourceId == trans.DestinationId && trans.SourceKind == trans.DestinationKind {
		log.Error("SourceId, SourceKind should not equal DestinationID, DestinationKind, set to '%v','%v'", trans.SourceId, trans.SourceKind, c)
		http.Fail(c, 500, ErrorCircularTransaction.Error(), ErrorCircularTransaction)
		return
	}

	if trans.Amount == currency.Cents(0) {
		log.Error(ErrorPointlessTransaction.Error(), c)
		http.Fail(c, 500, ErrorPointlessTransaction.Error(), ErrorPointlessTransaction)
		return
	}

	if trans.Currency == "" {
		log.Error(ErrorCurrencyRequired.Error(), c)
		http.Fail(c, 500, ErrorCurrencyRequired.Error(), ErrorCurrencyRequired)
		return
	}

	if !org.Live {
		log.Info("Transaction created in test mode.", c)
		trans.Test = true
	}

	err := db.RunInTransaction(func(db *datastore.Datastore) error {
		if trans.Type == transaction.Transfer || trans.Type == transaction.Withdraw {
			datas, err := util.GetTransactionsByCurrency(db.Context, trans.SourceId, trans.SourceKind, trans.Currency, !org.Live)
			if err != nil {
				return err
			}

			if trans.SourceId == "" || trans.SourceKind == "" {
				log.Error("SourceId and SourceKind are required, provided with '%v', '%v'", trans.SourceId, trans.SourceKind, c)
				return ErrorSourceRequired
			}

			if trans.Type == transaction.Transfer && (trans.DestinationId == "" || trans.DestinationKind == "") {
				log.Error("DestinationId and DestinationKind are required, provided with '%v', '%v'", trans.DestinationId, trans.DestinationKind, c)
				return ErrorDestinationRequired
			} else if trans.Type == transaction.Withdraw {
				// Withdraw has no destination
				trans.DestinationId = ""
				trans.DestinationKind = ""
			}

			data := datas.Data[trans.Currency]

			if data == nil {
				log.Error("Source has no funds'%v'", c)
				return ErrorInsufficientFunds
			}

			if data.Balance-data.Holds < trans.Amount {
				log.Error("Source has insufficient funds '%v' - '%v' < '%v'", data.Balance, data.Holds, trans.Amount, c)
				return ErrorInsufficientFunds
			}
		} else if trans.Type == transaction.Deposit {
			trans.SourceId = ""
			trans.SourceKind = ""

			if trans.DestinationId == "" || trans.DestinationKind == "" {
				log.Error("DestinationId and DestinationKind are required, provided with '%v', '%v'", trans.DestinationId, trans.DestinationKind, c)
				return ErrorDestinationRequired
			}
		}
		return trans.Create()
	})

	if err != nil {
		http.Fail(c, 500, err.Error(), err)
		return
	} else {
		c.Writer.Header().Add("Location", c.Request.URL.Path+"/"+trans.Id())
		http.Render(c, 201, trans)
	}
}

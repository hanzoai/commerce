package account

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/api/organization/wallet"
	"hanzo.io/datastore"
	"hanzo.io/middleware"
	"hanzo.io/models/blockchains"
	"hanzo.io/models/transaction"
	"hanzo.io/models/transaction/util"
	"hanzo.io/models/types/currency"
	"hanzo.io/util/blockchain"
	"hanzo.io/util/json"
	"hanzo.io/util/json/http"
	"hanzo.io/util/log"
)

func withdraw(c *gin.Context) {
	org := middleware.GetOrganization(c)
	usr := middleware.GetUser(c)

	db := datastore.New(c)

	// Get hte wallet
	orgWallet, err := wallet.ReturnWallet(org, db)
	if err != nil {
		http.Fail(c, 400, "Unable to retrieve wallet", err)
		return
	}

	// Decode the request
	request := wallet.PayFromAccountRequest{}
	if err := json.Decode(c.Request.Body, &request); err != nil {
		http.Fail(c, 400, "Failed to decode request body", err)
		return
	}

	// Account on the org should be publically avaiable and withdrawable
	account, success := orgWallet.GetAccountByName(request.Name)
	if !account.Withdrawable || !success {
		if !success {
			log.Error("Account %s does not exist", request.Name)
		}
		if !account.Withdrawable {
			log.Error("Account %s does is not withdrawable", request.Name)
		}
		http.Fail(c, 400, "Account not withdrawable", ErrorAccountNotWithdrawable)
		return
	}

	// Determine the currency
	var cur currency.Type

	switch account.Type {
	case blockchains.EthereumType, blockchains.EthereumRopstenType:
		cur = currency.ETH
	// Bitcoin accounts use BTC
	case blockchains.BitcoinType, blockchains.BitcoinTestnetType:
		cur = currency.BTC
	}

	var transactionId string

	nsDb := usr.Db

	// Check against the balance
	err = nsDb.RunInTransaction(func(db *datastore.Datastore) error {
		datas, err := util.GetTransactionsByCurrency(nsDb.Context, usr.Id(), "user", cur, !org.Live)
		if err != nil {
			return err
		}

		data, ok := datas.Data[cur]
		if !ok {
			log.Error("Source has no funds'%v'", c)
			return ErrorInsufficientFunds
		}

		if data.Balance-data.Holds < request.Amount {
			log.Error("Source has insufficient funds '%v' - '%v' < '%v'", data.Balance, data.Holds, request.Amount, c)
			return ErrorInsufficientFunds
		}

		transactionId, err = blockchain.MakePayment(middleware.GetAppEngine(c), *account, request.To, request.Amount, request.Fee, []byte(org.WalletPassphrase))
		if err != nil {
			log.Error("Failed to create transaction %v", err)
			return err
		}

		trans := transaction.New(nsDb)
		trans.SourceId = usr.Id()
		trans.SourceKind = "user"
		trans.Amount = request.Amount
		trans.Currency = cur
		trans.Type = transaction.Withdraw

		return trans.Create()
	})

	if err != nil {
		http.Fail(c, 500, err.Error(), err)
		return
	} else {
		http.Render(c, 200, wallet.PayFromAccountResponse{transactionId})
	}
}

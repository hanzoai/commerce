package account

import (
	"errors"
	"github.com/gin-gonic/gin"

	"hanzo.io/api/organization/wallet"
	"hanzo.io/datastore"
	"hanzo.io/middleware"
	"hanzo.io/util/blockchain"
	"hanzo.io/util/json"
	"hanzo.io/util/json/http"
)

func withdraw(c *gin.Context) {
	org := middleware.GetOrganization(c)

	db := datastore.New(c)
	orgWallet, err := wallet.ReturnWallet(org, db)
	if err != nil {
		http.Fail(c, 400, "Unable to retrieve wallet from datastore", err)
	}
	request := wallet.PayFromAccountRequest{}
	if err := json.Decode(c.Request.Body, &request); err != nil {
		http.Fail(c, 400, "Failed to decode request body", err)
		return
	}
	account, success := orgWallet.GetAccountByName(request.Name)
	if account.Withdrawable {

	}

	if !success {
		http.Fail(c, 404, "Requested account name was not found.", errors.New("Requested account name was not found."))
		return
	}
	transactionId, err := blockchain.MakePayment(middleware.GetAppEngine(c), *account, request.To, request.Amount, request.Fee, []byte(org.WalletPassphrase))
	if err != nil {
		http.Fail(c, 400, "Failed to make payment.", err)
		return
	}
	org.MustUpdate()

	http.Render(c, 200, wallet.PayFromAccountResponse{transactionId})
}

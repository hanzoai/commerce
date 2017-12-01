package wallet

import (
	"appengine"
	"errors"
	"github.com/gin-gonic/gin"
	"math/big"

	"hanzo.io/datastore"
	"hanzo.io/middleware"
	"hanzo.io/models/blockchains"
	"hanzo.io/util/blockchain"
	"hanzo.io/util/json"
	"hanzo.io/util/json/http"
)

type CreateWalletRequest struct {
	Name       string `json:"name"`
	Blockchain string `json:"blockchain"`
	Password   string `json:"password"`
}

type PayFromAccountRequest struct {
	Name     string  `json:"name"`
	To       string  `json:"to"`
	Amount   big.Int `json:"amount"`
	Password string  `json:"password"`
}

type PayFromAccountResponse struct {
	TransactionId string `json:"transactionId"`
}

func Get(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(c)
	orgWallet, err := org.GetOrCreateWallet(db)
	if err != nil {
		http.Fail(c, 400, "Unable to retrieve wallet from datastore", err)
	}
	http.Render(c, 200, orgWallet)
}

func GetAccount(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(c)
	orgWallet, err := org.GetOrCreateWallet(db)
	if err != nil {
		http.Fail(c, 400, "Unable to retrieve wallet from datastore", err)
	}
	account, success := orgWallet.GetAccountByName(c.Params.ByName("name"))
	if !success {
		http.Fail(c, 404, "Requested account name was not found.", errors.New("Requested account name was not found."))
		return
	}
	http.Render(c, 200, account)
}

func CreateAccount(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(c)
	orgWallet, err := org.GetOrCreateWallet(db)
	if err != nil {
		http.Fail(c, 400, "Unable to retrieve wallet from datastore", err)
	}
	request := CreateWalletRequest{}
	if err := json.Decode(c.Request.Body, &request); err != nil {
		http.Fail(c, 400, "Failed to decode request body", err)
		return
	}
	account, err := orgWallet.CreateAccount(request.Name, blockchains.Type(request.Blockchain), []byte(request.Password))
	if err != nil {
		http.Fail(c, 400, "Failed to create requested account", err)
		return
	}
	http.Render(c, 200, account)
}

func Pay(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(c)
	orgWallet, err := org.GetOrCreateWallet(db)
	if err != nil {
		http.Fail(c, 400, "Unable to retrieve wallet from datastore", err)
	}
	request := PayFromAccountRequest{}
	if err := json.Decode(c.Request.Body, &request); err != nil {
		http.Fail(c, 400, "Failed to decode request body", err)
		return
	}
	account, success := orgWallet.GetAccountByName(request.Name)
	if !success {
		http.Fail(c, 404, "Requested account name was not found.", errors.New("Requested account name was not found."))
		return
	}
	transactionId, err := blockchain.MakePayment(appengine.NewContext(c.Request), *account, request.To, &request.Amount, []byte(request.Password))
	if err != nil {
		http.Fail(c, 400, "Failed to make payment.", err)
		return
	}
	http.Render(c, 200, PayFromAccountResponse{transactionId})
}

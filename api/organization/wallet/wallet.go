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
	"hanzo.io/util/log"
)

type CreateAccountRequest struct {
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
	if err != nil || orgWallet == nil {
		http.Fail(c, 400, "Unable to retrieve wallet from datastore", err)
	}
	org.MustUpdate()

	http.Render(c, 200, orgWallet)
}

func GetAccount(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(c)
	orgWallet, err := org.GetOrCreateWallet(db)
	if err != nil {
		http.Fail(c, 400, "Unable to retrieve wallet from datastore", err)
	}
	log.Debug("Requested account name: %v", c.Params.ByName("name"))
	account, success := orgWallet.GetAccountByName(c.Params.ByName("name"))
	if !success {
		http.Fail(c, 404, "Requested account name was not found.", errors.New("Requested account name was not found."))
		return
	}
	org.MustUpdate()

	http.Render(c, 200, account)
}

func CreateAccount(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(c)
	orgWallet, err := org.GetOrCreateWallet(db)
	if err != nil {
		http.Fail(c, 400, "Unable to retrieve wallet from datastore", err)
	}
	request := CreateAccountRequest{}
	if err := json.Decode(c.Request.Body, &request); err != nil {
		http.Fail(c, 400, "Failed to decode request body", err)
		return
	}
	log.Debug("Blockchain requested for account creation: %v", request.Blockchain)
	blockchainType := blockchains.Type(request.Blockchain)
	account, err := orgWallet.CreateAccount(request.Name, blockchainType, []byte(request.Password))
	if err != nil {
		http.Fail(c, 400, "Failed to create requested account", err)
		return
	}
	org.MustUpdate()

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
	org.MustUpdate()

	http.Render(c, 200, PayFromAccountResponse{transactionId})
}

package wallet

import (
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"math/big"

	"hanzo.io/datastore"
	"hanzo.io/middleware"
	"hanzo.io/models/blockchains"
	"hanzo.io/models/user"
	"hanzo.io/models/wallet"
	"hanzo.io/util/blockchain"
	"hanzo.io/util/json"
	"hanzo.io/util/json/http"
	"hanzo.io/util/log"
	"hanzo.io/util/rand"
)

type CreateAccountRequest struct {
	Name       string `json:"name"`
	Blockchain string `json:"blockchain"`
}

type PayFromAccountRequest struct {
	Name   string `json:"name"`
	To     string `json:"to"`
	Amount string `json:"amount"`
}

type PayFromAccountResponse struct {
	TransactionId string `json:"transactionId"`
}

func Get(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))
	id := c.Params.ByName("userid")

	u := user.New(db)
	if err := u.GetById(id); err != nil {
		http.Fail(c, 400, "Could not query user", err)
		return
	}

	userWallet, err := returnWallet(u, db)
	if err != nil || userWallet == nil {
		http.Fail(c, 400, "Unable to user retrieve wallet from datastore", err)
	}
	u.MustUpdate()

	http.Render(c, 200, userWallet)
}

func GetAccount(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))
	id := c.Params.ByName("userid")

	u := user.New(db)
	if err := u.GetById(id); err != nil {
		http.Fail(c, 400, "Could not query user", err)
		return
	}

	userWallet, err := returnWallet(u, db)
	if err != nil {
		http.Fail(c, 400, "Unable to retrieve wallet from datastore", err)
	}
	log.Debug("Requested account name: %v", c.Params.ByName("name"))
	account, success := userWallet.GetAccountByName(c.Params.ByName("name"))
	if !success {
		http.Fail(c, 404, "Requested account name was not found.", errors.New("Requested account name was not found."))
		return
	}
	u.MustUpdate()
	http.Render(c, 200, account)
}

func CreateAccount(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))
	id := c.Params.ByName("userid")

	u := user.New(db)
	if err := u.GetById(id); err != nil {
		http.Fail(c, 400, "Could not query user", err)
		return
	}

	userWallet, err := returnWallet(u, db)
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
	account, err := userWallet.CreateAccount(request.Name, blockchainType, []byte(u.WalletKey))
	if err != nil {
		http.Fail(c, 400, "Failed to create requested account", err)
		return
	}
	u.MustUpdate()

	http.Render(c, 200, account)
}

func Pay(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))
	id := c.Params.ByName("userid")

	u := user.New(db)
	if err := u.GetById(id); err != nil {
		http.Fail(c, 400, "Could not query user", err)
		return
	}

	userWallet, err := returnWallet(u, db)
	if err != nil {
		http.Fail(c, 400, "Unable to retrieve wallet from datastore", err)
	}
	request := PayFromAccountRequest{}
	if err := json.Decode(c.Request.Body, &request); err != nil {
		http.Fail(c, 400, "Failed to decode request body", err)
		return
	}
	value := new(big.Int)
	_, success := value.SetString(request.Amount, 10)
	if !success {
		http.Fail(c, 400, "Failed to decode value. Must be parsable as base 10 string.", errors.New(fmt.Sprintf("Unable to decode value. Given value: %v", request.Amount)))
	}

	account, success := userWallet.GetAccountByName(request.Name)
	if !success {
		http.Fail(c, 404, "Requested account name was not found.", errors.New("Requested account name was not found."))
		return
	}
	transactionId, err := blockchain.MakePayment(middleware.GetAppEngine(c), *account, request.To, value, []byte(u.WalletKey))
	if err != nil {
		http.Fail(c, 400, "Failed to make payment.", err)
		return
	}
	u.MustUpdate()

	http.Render(c, 200, PayFromAccountResponse{transactionId})
}

func returnWallet(u *user.User, db *datastore.Datastore) (*wallet.Wallet, error) {
	ret, err := u.GetOrCreateWallet(db)
	if err != nil {
		return nil, err
	}
	if u.WalletKey == "" {
		u.WalletKey = rand.SecretKey()
	}

	return ret, nil
}

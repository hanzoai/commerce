package wallet

import (
	"errors"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/blockchains"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/models/user"
	"github.com/hanzoai/commerce/models/wallet"
	"github.com/hanzoai/commerce/util/blockchain"
	"github.com/hanzoai/commerce/util/json"
	"github.com/hanzoai/commerce/util/json/http"
	"github.com/hanzoai/commerce/util/rand"
)

type CreateAccountRequest struct {
	Name       string `json:"name"`
	Blockchain string `json:"blockchain"`
}

type PayFromAccountRequest struct {
	Name   string         `json:"name"`
	To     string         `json:"to"`
	Amount currency.Cents `json:"amount"`
	Fee    currency.Cents `json:"fee"`
}

type PayFromAccountResponse struct {
	TransactionId string `json:"transactionId"`
}

func Get(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))
	id := c.Param("userid")

	u := user.New(db)
	if err := u.GetById(id); err != nil {
		return http.Fail(c, 400, "Could not query user", err)
	}

	userWallet, err := returnWallet(u, db)
	if err != nil || userWallet == nil {
		return http.Fail(c, 400, "Unable to user retrieve wallet from datastore", err)
	}
	u.MustUpdate()

	return http.Render(c, 200, userWallet)
}

func GetAccount(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))
	id := c.Param("userid")

	u := user.New(db)
	if err := u.GetById(id); err != nil {
		return http.Fail(c, 400, "Could not query user", err)
	}

	userWallet, err := returnWallet(u, db)
	if err != nil {
		return http.Fail(c, 400, "Unable to retrieve wallet from datastore", err)
	}
	log.Debug("Requested account name: %v", c.Param("name"))
	account, success := userWallet.GetAccountByName(c.Param("name"))
	if !success {
		return http.Fail(c, 404, "Requested account name was not found.", errors.New("Requested account name was not found."))
	}
	u.MustUpdate()
	return http.Render(c, 200, account)
}

func CreateAccount(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))
	id := c.Param("userid")

	u := user.New(db)
	if err := u.GetById(id); err != nil {
		return http.Fail(c, 400, "Could not query user", err)
	}

	userWallet, err := returnWallet(u, db)
	if err != nil {
		return http.Fail(c, 400, "Unable to retrieve wallet from datastore", err)
	}
	request := CreateAccountRequest{}
	if err := json.DecodeBytes(c.Body(), &request); err != nil {
		return http.Fail(c, 400, "Failed to decode request body", err)
	}
	log.Debug("Blockchain requested for account creation: %v", request.Blockchain)
	blockchainType := blockchains.Type(request.Blockchain)
	account, err := userWallet.CreateAccount(request.Name, blockchainType, []byte(u.WalletPassphrase))
	if err != nil {
		return http.Fail(c, 400, "Failed to create requested account", err)
	}
	u.MustUpdate()

	return http.Render(c, 200, account)
}

func Send(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))
	id := c.Param("userid")

	u := user.New(db)
	if err := u.GetById(id); err != nil {
		return http.Fail(c, 400, "Could not query user", err)
	}

	userWallet, err := returnWallet(u, db)
	if err != nil {
		return http.Fail(c, 400, "Unable to retrieve wallet from datastore", err)
	}
	request := PayFromAccountRequest{}
	if err := json.DecodeBytes(c.Body(), &request); err != nil {
		return http.Fail(c, 400, "Failed to decode request body", err)
	}

	account, success := userWallet.GetAccountByName(request.Name)
	if !success {
		return http.Fail(c, 404, "Requested account name was not found.", errors.New("Requested account name was not found."))
	}
	transactionId, err := blockchain.MakePayment(c.Context(), *account, request.To, request.Amount, request.Fee, []byte(u.WalletPassphrase))
	if err != nil {
		return http.Fail(c, 400, "Failed to make payment.", err)
	}
	u.MustUpdate()

	return http.Render(c, 200, PayFromAccountResponse{transactionId})
}

func returnWallet(u *user.User, db *datastore.Datastore) (*wallet.Wallet, error) {
	ret, err := u.GetOrCreateWallet(db)
	if err != nil {
		return nil, err
	}
	if u.WalletPassphrase == "" {
		u.WalletPassphrase = rand.SecretKey()
	}

	return ret, nil
}

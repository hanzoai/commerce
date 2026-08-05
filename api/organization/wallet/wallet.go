package wallet

import (
	"errors"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/blockchains"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/types/currency"
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
	// GasPrice of Fee Per Byte
	Fee currency.Cents `json:"fee"`
}

type PayFromAccountResponse struct {
	TransactionId string `json:"transactionId"`
}

func Get(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	orgWallet, err := ReturnWallet(org)
	if err != nil || orgWallet == nil {
		return http.Fail(c, 400, "Unable to retrieve wallet from datastore", err)
	}
	org.MustUpdate()

	return http.Render(c, 200, orgWallet)
}

func GetAccount(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	orgWallet, err := ReturnWallet(org)
	if err != nil {
		return http.Fail(c, 400, "Unable to retrieve wallet from datastore", err)
	}

	log.Debug("Requested account name: %v", c.Param("name"))
	account, success := orgWallet.GetAccountByName(c.Param("name"))
	if !success {
		return http.Fail(c, 404, "Requested account name was not found.", errors.New("Requested account name was not found."))
	}
	org.MustUpdate()

	return http.Render(c, 200, account)
}

func CreateAccount(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	orgWallet, err := ReturnWallet(org)
	if err != nil {
		return http.Fail(c, 400, "Unable to retrieve wallet from datastore", err)
	}
	request := CreateAccountRequest{}
	if err := json.DecodeBytes(c.Body(), &request); err != nil {
		return http.Fail(c, 400, "Failed to decode request body", err)
	}
	log.Debug("Blockchain requested for account creation: %v", request.Blockchain)
	blockchainType := blockchains.Type(request.Blockchain)
	account, err := orgWallet.CreateAccount(request.Name, blockchainType, []byte(org.WalletPassphrase))
	if err != nil {
		return http.Fail(c, 400, "Failed to create requested account", err)
	}
	org.MustUpdate()

	return http.Render(c, 200, account)
}

func Send(c *zip.Ctx) error {
	// Crypto withdrawal from the org wallet — the highest-value money move.
	// Admin-only, enforced inside the handler because the route middleware
	// no-ops on the IAM path (Red HIGH-4). The wallet lives on the caller's own
	// org record (resolved from the verified X-Org-Id), so it is already
	// tenant-scoped; this closes the missing authorization check.
	if !middleware.RequireAdmin(c) {
		return nil
	}
	org := middleware.GetOrganization(c)
	orgWallet, err := ReturnWallet(org)
	if err != nil {
		return http.Fail(c, 400, "Unable to retrieve wallet from datastore", err)
	}
	request := PayFromAccountRequest{}
	if err := json.DecodeBytes(c.Body(), &request); err != nil {
		return http.Fail(c, 400, "Failed to decode request body", err)
	}
	account, success := orgWallet.GetAccountByName(request.Name)
	if !success {
		return http.Fail(c, 404, "Requested account name was not found.", errors.New("Requested account name was not found."))
	}
	transactionId, err := blockchain.MakePayment(c.Context(), *account, request.To, request.Amount, request.Fee, []byte(org.WalletPassphrase))
	if err != nil {
		return http.Fail(c, 400, "Failed to make payment.", err)
	}
	org.MustUpdate()

	return http.Render(c, 200, PayFromAccountResponse{transactionId})
}

func ReturnWallet(o *organization.Organization) (*wallet.Wallet, error) {
	ret, err := o.GetOrCreateWallet(o.Datastore())
	if err != nil {
		return nil, err
	}
	if o.WalletPassphrase == "" {
		o.WalletPassphrase = rand.SecretKey()
	}

	return ret, nil
}

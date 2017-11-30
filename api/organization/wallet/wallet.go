package wallet

import (
	"errors"
	"github.com/gin-gonic/gin"

	"hanzo.io/middleware"
	"hanzo.io/models/blockchains"
	"hanzo.io/util/json"
	"hanzo.io/util/json/http"
)

type CreateWalletRequest struct {
	Name       string
	Blockchain string
	Password   string
}

func Get(c *gin.Context) {
	org := middleware.GetOrganization(c)
	orgWallet := org.Wallet
	http.Render(c, 200, orgWallet)
}

func GetAccount(c *gin.Context) {
	org := middleware.GetOrganization(c)
	orgWallet := org.Wallet
	account, success := orgWallet.GetAccountByName(c.Params.ByName("name"))
	if !success {
		http.Fail(c, 400, "Failed to retrieve requested account name.", errors.New("Failed to retrieve requestd account name."))
		return
	}
	http.Render(c, 200, account)
}

func CreateAccount(c *gin.Context) {
	org := middleware.GetOrganization(c)
	orgWallet := org.Wallet
	request := CreateWalletRequest{}
	if err := json.Decode(c.Request.Body, &request); err != nil {
		http.Fail(c, 400, "Failed decode request body", err)
		return
	}
	account, err := orgWallet.CreateAccount(request.Name, blockchains.Type(request.Blockchain), []byte(request.Password))
	if err != nil {
		http.Fail(c, 400, "Failed decode request body", err)
		return
	}
	http.Render(c, 200, account)

}

func Pay(c *gin.Context) {
	org := middleware.GetOrganization(c)
	orgWallet := org.Wallet
	http.Render(c, 200, orgWallet)
}

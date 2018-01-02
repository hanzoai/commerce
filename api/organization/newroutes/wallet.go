package newroutes

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/middleware"
	"hanzo.io/models/blockchains"
	"hanzo.io/models/types/currency"
	"hanzo.io/util/json/http"
)

type AccountNameRes struct {
	Name     string        `json:"name"`
	Currency currency.Type `json:"currency"`
}

type GetWithdrawableAccountsRes struct {
	Accounts []AccountNameRes `json:"accounts"`
}

func GetWithdrawableAccounts(c *gin.Context) {
	org := middleware.GetOrganization(c)

	// Create the response object
	res := GetWithdrawableAccountsRes{make([]AccountNameRes, 0, 0)}

	// Wallet doesn't exist so return empty response object
	if org.WalletId == "" {
		http.Render(c, 200, res)
		return
	}

	// Fetch the wallet
	w, err := org.GetOrCreateWallet(org.Db)
	if err != nil {
		http.Fail(c, 400, "Failed to lookup wallets", err)
	}

	// Loop over accounts
	for _, a := range w.Accounts {
		if a.Withdrawable {
			anr := AccountNameRes{
				Name: a.Name,
			}

			switch a.Type {
			// Ethereum accounts use ETH
			case blockchains.EthereumType, blockchains.EthereumRopstenType:
				anr.Currency = currency.ETH
			// Bitcoin accounts use BTC
			case blockchains.BitcoinType, blockchains.BitcoinTestnetType:
				anr.Currency = currency.BTC
			// Don't report unsupported blockchains
			default:
				continue
			}
			res.Accounts = append(res.Accounts, anr)
		}
	}

	http.Render(c, 200, res)
}

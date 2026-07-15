package newroutes

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/blockchains"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/json"
	"github.com/hanzoai/commerce/util/json/http"
)

type AccountNameRes struct {
	Name     string           `json:"name"`
	Type     blockchains.Type `json:"type"`
	Currency currency.Type    `json:"currency"`
}

type GetWithdrawableAccountsRes struct {
	Accounts []AccountNameRes `json:"accounts"`
}

func GetWithdrawableAccounts(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)

	// Create the response object
	res := GetWithdrawableAccountsRes{make([]AccountNameRes, 0, 0)}

	// Wallet doesn't exist so return empty response object
	if org.WalletId == "" {
		return http.Render(c, 200, res)
	}

	// Fetch the wallet
	w, err := org.GetOrCreateWallet(org.Datastore())
	if err != nil {
		return http.Fail(c, 400, "Failed to lookup wallets", err)
	}

	// Loop over accounts
	for _, a := range w.Accounts {
		if a.Withdrawable {
			log.Error("account %v withdrawable", a)
			anr := AccountNameRes{
				Name: a.Name,
				Type: a.Type,
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
		} else {
			log.Error("account %v not withdrawable", a)
		}
	}

	log.Error("res %v", json.Encode(res))

	return http.Render(c, 200, res)
}

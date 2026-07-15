package ethereum

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/payment"
	"github.com/hanzoai/commerce/util/json/http"
)

type FromToFinal struct {
	TxHash      string `json:"txHash"`
	FinalTxHash string `json:"finalTxHash"`
	From        string `json:"from"`
	To          string `json:"to"`
	Final       string `json:"final"`
}

func Lookup(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)

	// Set up the db with the namespaced context
	ctx := org.Namespaced(c.Context())
	db := datastore.NewNamespaced(ctx) // per-org payment store (Red MED-1)

	proxyAddress := c.Param("proxyaddress")

	pay := payment.New(db)
	if ok, err := pay.Query().Filter("Account.EthereumToAddress=", proxyAddress).Get(); !ok {
		log.Warn("Failed to find Ethereum Proxy Address", err, c)
		return http.Fail(c, 404, "Failed to find Ethereum Proxy Address", err)
	}

	return http.Render(c, 200, FromToFinal{
		From:        pay.Account.EthereumFromAddress,
		To:          pay.Account.EthereumToAddress,
		Final:       pay.Account.EthereumFinalAddress,
		TxHash:      pay.Account.EthereumTransactionHash,
		FinalTxHash: pay.Account.EthereumFinalTransactionHash,
	})
}

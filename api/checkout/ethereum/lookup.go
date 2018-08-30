package ethereum

import(
	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/middleware"
	"hanzo.io/models/payment"
	"hanzo.io/log"
	"hanzo.io/util/json/http"
)

type FromToFinal struct {
	TxHash string `json:"txHash"`
	FinalTxHash string `json:"finalTxHash"`
	From string `json:"from"`
	To string `json:"to"`
	Final string `json:"final"`
}

func Lookup(c *gin.Context) {
	org := middleware.GetOrganization(c)

	// Set up the db with the namespaced appengine context
	ctx := org.Namespaced(c)
	db := datastore.New(ctx)

	proxyAddress := c.Params.ByName("proxyaddress")

	pay := payment.New(db)
	if ok, err := pay.Query().Filter("EthereumToAddress=", proxyAddress).Get(); !ok {
		http.Fail(c, 404, "Failed to find Ethereum Proxy Address", err)
		log.Warn("Failed to find Ethereum Proxy Address", err, c)
		return
	}

	http.Render(c, 200, FromToFinal{
		From: pay.Account.EthereumFromAddress,
		To: pay.Account.EthereumToAddress,
		Final: pay.Account.EthereumFinalAddress,
		TxHash: pay.Account.EthereumTransactionHash,
		FinalTxHash: pay.Account.EthereumFinalTransactionHash,
	})
}


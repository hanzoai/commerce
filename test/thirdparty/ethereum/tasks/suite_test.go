package test

import (
	"testing"

	"github.com/gin-gonic/gin"

	"appengine"

	"hanzo.io/datastore"
	"hanzo.io/models/blockchains"
	"hanzo.io/models/fixtures"
	"hanzo.io/models/namespace"
	"hanzo.io/models/order"
	"hanzo.io/models/types/currency"
	"hanzo.io/models/user"
	"hanzo.io/models/wallet"
	"hanzo.io/util/gincontext"
	"hanzo.io/util/rand"
	"hanzo.io/util/test/ae"

	. "hanzo.io/util/test/ginkgo"
)

var (
	ctx  ae.Context
	c    *gin.Context
	db   *datastore.Datastore
	nsDb *datastore.Datastore
	ord  *order.Order
	usr  *user.User
	w    *wallet.Wallet
)

func Test(t *testing.T) {
	Setup("thirdparty/ethereum/tasks", t)
}

var _ = BeforeSuite(func() {
	ctx = ae.NewContext()
	c = gincontext.New(ctx)
	db = datastore.New(ctx)

	ns := namespace.New(db)
	ns.Name = "test"
	ns.GetOrCreate("Name=", "test")

	fixtures.BlockchainNamespace(c)

	nsCtx, err := appengine.Namespace(ctx, "test")
	if err != nil {
		panic(err)
	}

	nsDb = datastore.New(nsCtx)

	ord = order.New(nsDb)
	ord.Currency = currency.ETH
	ord.Total = 123
	ord.WalletPassphrase = rand.SecretKey()
	ord.Test = true

	usr = user.New(nsDb)
	usr.FirstName = "Test"
	usr.LastName = "User"
	usr.MustCreate()

	ord.UserId = usr.Id()

	w, err = ord.GetOrCreateWallet(ord.Db)
	w.CreateAccount("Receiver Account", blockchains.EthereumType, []byte(ord.WalletPassphrase))

	ord.MustCreate()
})

var _ = AfterSuite(func() {
	ctx.Close()
})

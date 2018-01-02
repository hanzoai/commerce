package test

import (
	"testing"

	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/models/blockchains"
	"hanzo.io/models/blockchains/blocktransaction"
	"hanzo.io/models/fixtures"
	"hanzo.io/models/order"
	"hanzo.io/models/organization"
	"hanzo.io/models/types/currency"
	"hanzo.io/models/user"
	"hanzo.io/models/wallet"
	"hanzo.io/thirdparty/bitcoin"
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
	org  *organization.Organization
	usr  *user.User
	w    *wallet.Wallet
	pw   *wallet.Wallet
)

func Test(t *testing.T) {
	Setup("thirdparty/bitcoin/tasks", t)
}

var _ = BeforeSuite(func() {
	ctx = ae.NewContext()
	c = gincontext.New(ctx)
	db = datastore.New(ctx)

	org = fixtures.Organization(c).(*organization.Organization)
	pw = fixtures.PlatformWallet(c).(*wallet.Wallet)

	nsCtx := org.Namespaced(ctx)
	nsDb = datastore.New(nsCtx)

	ord = order.New(nsDb)
	ord.Currency = currency.ETH
	ord.Total = 123 * 1e6
	ord.WalletPassphrase = rand.SecretKey()
	ord.Test = true

	usr = user.New(nsDb)
	usr.FirstName = "Test"
	usr.LastName = "User"
	usr.MustCreate()

	ord.UserId = usr.Id()

	w, _ = ord.GetOrCreateWallet(ord.Db)

	ord.MustCreate()

	bitcoin.Test(true)

	// Create Receiver
	ra, err := w.CreateAccount("Receiver Account", blockchains.BitcoinTestnetType, []byte(ord.WalletPassphrase))
	Expect(err).NotTo(HaveOccurred())

	// Create a Blocktransaction
	bt := blocktransaction.New(db)
	bt.Type = blockchains.BitcoinTestnetType
	bt.BitcoinTransactionType = blockchains.BitcoinTransactionTypeVOut
	bt.BitcoinTransactionUsed = false
	bt.BitcoinTransactionVOutValue = int64(123e6)
	bt.BitcoinTransactionTxId = "0"
	bt.BitcoinTransactionVOutIndex = 0
	bt.Address = ra.Address
	bt.MustCreate()
})

var _ = AfterSuite(func() {
	ctx.Close()
})

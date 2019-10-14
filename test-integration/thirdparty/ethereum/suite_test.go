package test

import (
	"testing"

	"github.com/gin-gonic/gin"

	"hanzo.io/config"
	"hanzo.io/datastore"
	// "hanzo.io/models/fixtures"
	"hanzo.io/models/wallet"
	"hanzo.io/thirdparty/ethereum"
	"hanzo.io/util/gincontext"
	"hanzo.io/util/test/ae"

	. "hanzo.io/util/test/ginkgo"
)

var (
	ctx    ae.Context
	c      *gin.Context
	db     *datastore.Datastore
	client ethereum.Client
	w      *wallet.Wallet
)

func Test(t *testing.T) {
	Setup("thirdparty/ethereum", t)
}

// Can't actually test without mocking because we can't regenerate wallets

var _ = BeforeSuite(func() {
	ctx = ae.NewContext()
	c = gincontext.New(ctx)
	db = datastore.New(ctx)

	// w = fixtures.PlatformWallet(c).(*wallet.Wallet)

	client = ethereum.New(ctx, config.Ethereum.TestNetNodes[0])
})

var _ = AfterSuite(func() {
	ctx.Close()
})

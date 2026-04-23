package test

import (
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/config"
	"github.com/hanzoai/commerce/datastore"
	// "github.com/hanzoai/commerce/models/fixtures"
	"github.com/hanzoai/commerce/models/wallet"
	"github.com/hanzoai/commerce/thirdparty/ethereum"
	"github.com/hanzoai/commerce/util/gincontext"
	"github.com/hanzoai/commerce/util/test/ae"

	. "github.com/hanzoai/commerce/util/test/ginkgo"
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

package test

import (
	"testing"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/fixtures"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/util/test/ae"
	"github.com/hanzoai/commerce/util/zipctx"

	"github.com/hanzoai/commerce/thirdparty/authorizenet"

	. "github.com/hanzoai/commerce/util/test/ginkgo"
)

var (
	c      *zip.Ctx
	ctx    ae.Context
	db     *datastore.Datastore
	client *authorizenet.Client
	token  string
)

func Test(t *testing.T) {
	Setup("thirdparty/authorizenet", t)
}

var _ = BeforeSuite(func() {
	ctx = ae.NewContext()
	c = zipctx.New(ctx)
	db = datastore.New(ctx)
	log.Warn("Before Suite")
	org := fixtures.Organization(c).(*organization.Organization)
	loginId := org.AuthorizeNet.Sandbox.LoginId
	transactionKey := org.AuthorizeNet.Sandbox.TransactionKey
	key := org.AuthorizeNet.Sandbox.Key
	client = authorizenet.New(ctx, loginId, transactionKey, key, true)
})

var _ = AfterSuite(func() {
	ctx.Close()
})

/*
* API Login Id:
* Transaction Key:
* Key: Simon
 */

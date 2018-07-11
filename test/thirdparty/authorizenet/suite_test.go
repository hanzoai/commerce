package test

import (
	"testing"

	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/models/fixtures"
	"hanzo.io/models/organization"
	"hanzo.io/util/gincontext"
	"hanzo.io/log"
	"hanzo.io/util/test/ae"

	"hanzo.io/thirdparty/authorizenet"

	. "hanzo.io/util/test/ginkgo"
)

var (
	c      *gin.Context
	ctx    ae.Context
	db     *datastore.Datastore
	client *authorizenet.Client
	token string
)

func Test(t *testing.T) {
	Setup("thirdparty/authorizenet", t)
}

var _ = BeforeSuite(func() {
	ctx = ae.NewContext()
	c = gincontext.New(ctx)
	db = datastore.New(c)
	log.Warn("Before Suite")
	org := fixtures.Organization(c).(*organization.Organization)
	loginId := org.AuthorizeNet.SandboxApiLoginId
	transactionKey := org.AuthorizeNet.SandboxTransactionKey
	key := org.AuthorizeNet.SandboxKey
	client = authorizenet.New(ctx, loginId, transactionKey, key)
})

var _ = AfterSuite(func() {
	ctx.Close()
})

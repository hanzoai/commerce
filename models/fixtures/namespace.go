package fixtures

import (
	"github.com/gin-gonic/gin"

	"appengine"

	"crowdstart.io/datastore"
	"crowdstart.io/middleware"
	"crowdstart.io/models/constants"
	"crowdstart.io/models/namespace"
)

var Namespace = New("namespace", func(c *gin.Context) *namespace.Namespace {
	ctx := middleware.GetAppEngine(c)
	nsCtx, err := appengine.Namespace(ctx, constants.NamespaceNamespace)
	if err != nil {
		panic(err)
	}

	nsDb := datastore.New(nsCtx)
	ns := namespace.New(nsDb)
	ns.StringId = constants.NamespaceRootKey
	ns.GetOrCreate("StringId=", constants.NamespaceRootKey)
	ns.IntId = ns.Key().IntID()
	ns.MustPut()

	return ns
})

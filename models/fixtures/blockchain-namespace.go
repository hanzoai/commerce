package fixtures

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/models/namespace"
	"hanzo.io/util/log"
)

var BlockchainName = "blockchains"

var BlockchainNamespace = New("blockchain-namespace", func(c *gin.Context) *namespace.Namespace {
	db := datastore.New(c)
	ns := namespace.New(db)
	err := ns.GetOrCreate("Name=", BlockchainName)

	if err != nil {
		log.Warn("Failed to put namespace: %v", err)
	}

	ns.Name = BlockchainName
	ns.IntId = 1234567890
	ns.MustUpdate()

	return ns
})

package fixtures

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/models/namespace"
	"hanzo.io/util/log"
)

var BlockchainName = "_blockchains"

var BlockchainNamespace = New("blockchain-namespace", func(c *gin.Context) *namespace.Namespace {
	db := datastore.New(c)
	ns := namespace.New(db)
	ns.Id_ = BlockchainName
	ns.Name = BlockchainName
	ns.IntId = 1234567890

	err := ns.GetOrCreate("Name=", BlockchainName)

	if err != nil {
		log.Warn("Failed to put namespace: %v", err)
	}

	ns.Id_ = BlockchainName
	ns.Name = BlockchainName
	ns.IntId = 1234567890
	ns.MustUpdate()

	return ns
})

package fixtures

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/models/namespace"
	"hanzo.io/log"

	"hanzo.io/models/blockchains"
)

var BlockchainNamespace = New("blockchain-namespace", func(c *context.Context) *namespace.Namespace {
	db := datastore.New(c)
	ns := namespace.New(db)
	ns.Id_ = blockchains.BlockchainNamespace
	ns.Name = blockchains.BlockchainNamespace
	ns.IntId = 1234567890

	err := ns.GetOrCreate("Name=", blockchains.BlockchainNamespace)

	if err != nil {
		log.Warn("Failed to put namespace: %v", err)
	}

	ns.Id_ = blockchains.BlockchainNamespace
	ns.Name = blockchains.BlockchainNamespace
	ns.IntId = 1234567890
	ns.MustUpdate()

	return ns
})

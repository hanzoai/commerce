package fixtures

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/namespace"

	"github.com/hanzoai/commerce/models/blockchains"
)

var BlockchainNamespace = New("blockchain-namespace", func(c *zip.Ctx) *namespace.Namespace {
	db := datastore.New(c.Context())
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

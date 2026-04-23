// Module hanzoai/commerce/thirdparty/ethereum is split out of the parent
// commerce module so consumers that don't touch EVM don't transitively pull
// luxfi/geth → luxfi/warp → luxfi/log. Only consumers that actually need
// Ethereum payment primitives import this sub-module and register themselves
// with the parent via init().
module github.com/hanzoai/commerce/thirdparty/ethereum

go 1.26.1

require (
	github.com/gin-gonic/gin v1.12.0
	github.com/hanzoai/commerce v1.40.0
	github.com/luxfi/crypto v1.17.45
	github.com/luxfi/geth v1.16.79
)

// The bare google.golang.org/genproto module was split into
// googleapis/api and googleapis/rpc submodules after mid-2023. Stale pre-split
// versions still get pulled transitively (via cloud.google.com/go/compute@v1.7.0),
// but nothing in this sub-module imports its packages. Pin to a post-split
// version so MVS never selects one that contains googleapis/api/httpbody.
require google.golang.org/genproto v0.0.0-20260319201613-d00831a3d3e7 // indirect
